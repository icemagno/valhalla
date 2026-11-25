package test;

import hla.rti1516e.*;
import hla.rti1516e.exceptions.*;

import java.nio.charset.StandardCharsets;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;

public class JavaFederateTest {

    public static void main(String[] args) {
        String crcAddress = args.length > 0 ? args[0] : "127.0.0.1:4000";
        System.out.println("==================================================");
        System.out.println("  VALHALLA NATIVE JAVA LRC SDK VERIFICATION TEST  ");
        System.out.println("  Target CRC: " + crcAddress);
        System.out.println("==================================================");

        try {
            RtiFactory factory = RtiFactoryFactory.getRtiFactory();
            RTIambassador rtiAlfa = factory.getRtiAmbassador();
            RTIambassador rtiBravo = factory.getRtiAmbassador();

            AtomicBoolean bravoDiscovered = new AtomicBoolean(false);
            AtomicInteger bravoReflectionCount = new AtomicInteger(0);

            FederateAmbassador fedAmbAlfa = new NullFederateAmbassador();

            FederateAmbassador fedAmbBravo = new NullFederateAmbassador() {
                @Override
                public void discoverObjectInstance(ObjectInstanceHandle theObject, ObjectClassHandle theObjectClass, String objectName) {
                    System.out.println(" -> [Bravo] Discovered Object: " + objectName + " (handle=" + theObject + ")");
                    bravoDiscovered.set(true);
                }

                @Override
                public void reflectAttributeValues(ObjectInstanceHandle theObject, AttributeHandleValueMap theAttributes, byte[] userSuppliedTag, OrderType sentOrdering, TransportationTypeHandle theTransport) {
                    System.out.println(" -> [Bravo] Reflected Attribute Values for object " + theObject + ", attrs=" + theAttributes.keySet());
                    for (var entry : theAttributes.entrySet()) {
                        String valStr = new String(entry.getValue(), StandardCharsets.UTF_8);
                        System.out.println("     Attr " + entry.getKey() + " => \"" + valStr + "\"");
                    }
                    bravoReflectionCount.incrementAndGet();
                }
            };

            System.out.println("[1] Connecting Federates Alfa and Bravo to Valhalla CRC...");
            rtiAlfa.connect(fedAmbAlfa, CallbackModel.HLA_EVOKED, crcAddress);
            rtiBravo.connect(fedAmbBravo, CallbackModel.HLA_EVOKED, crcAddress);
            System.out.println(" -> Both federates connected successfully.");

            String fedName = "JavaSdkTestFederation";
            System.out.println("[2] Creating Federation Execution: " + fedName);
            try {
                rtiAlfa.createFederationExecution(fedName);
                System.out.println(" -> Federation created.");
            } catch (FederationExecutionAlreadyExists e) {
                System.out.println(" -> Federation already exists, proceeding.");
            }

            System.out.println("[3] Joining Federation Execution...");
            FederateHandle handleAlfa = rtiAlfa.joinFederationExecution("AlfaFed", fedName);
            FederateHandle handleBravo = rtiBravo.joinFederationExecution("BravoFed", fedName);
            System.out.println(" -> Joined: Alfa handle=" + handleAlfa + ", Bravo handle=" + handleBravo);

            ObjectClassHandle jetClass = new ObjectClassHandle(1);
            AttributeHandle attrCoordinates = new AttributeHandle(1);

            AttributeHandleSet pubSet = new AttributeHandleSet();
            pubSet.add(attrCoordinates);

            System.out.println("[4] Publishing Object Class 1 with attribute [1] by Alfa...");
            rtiAlfa.publishObjectClassAttributes(jetClass, pubSet);

            System.out.println("[5] Configuring 3D Morton DDM Region on Bravo (0..500 in X, Y, Z)...");
            DimensionHandle[] dims = new DimensionHandle[]{
                    new DimensionHandle(1),
                    new DimensionHandle(2),
                    new DimensionHandle(3)
            };
            RegionHandle bravoRegion = rtiBravo.createRegion(dims);
            rtiBravo.setMetricBounds(bravoRegion, 0.0, 500.0, 0.0, 500.0, 0.0, 500.0);

            AttributeHandleSet subSet = new AttributeHandleSet();
            subSet.add(attrCoordinates);
            rtiBravo.subscribeObjectClassAttributesWithRegions(jetClass, subSet, bravoRegion);
            System.out.println(" -> Bravo subscribed to class 1 with 3D Morton region " + bravoRegion);

            System.out.println("[6] Alfa registers object instance 'SpatialFighterJet'...");
            ObjectInstanceHandle jetObj = rtiAlfa.registerObjectInstance(jetClass, "SpatialFighterJet");
            System.out.println(" -> Registered object handle=" + jetObj);

            // Evoke callbacks on Bravo to process discovery
            Thread.sleep(100);
            rtiBravo.evokeMultipleCallbacks(0.05, 0.2);
            if (!bravoDiscovered.get()) {
                throw new RuntimeException("Test Failed: Bravo did not discover object 'SpatialFighterJet'!");
            }
            System.out.println(" -> Discovery confirmed on Bravo!");

            System.out.println("[7] Testing DDM Spatial Filtering (Out-of-Bounds Update)...");
            AttributeHandleValueMap updateOut = new AttributeHandleValueMap();
            updateOut.put(attrCoordinates, "POS:1000,1000,1000".getBytes(StandardCharsets.UTF_8));
            rtiAlfa.updateAttributeValuesSpatial(jetObj, updateOut, 1000.0, 1000.0, 1000.0, new byte[]{0x01});

            Thread.sleep(100);
            rtiBravo.evokeMultipleCallbacks(0.05, 0.2);
            if (bravoReflectionCount.get() != 0) {
                throw new RuntimeException("Test Failed: Bravo received reflection for out-of-bounds position! DDM Morton failed!");
            }
            System.out.println(" -> DDM Filter SUCCESS: Out-of-bounds update was correctly filtered!");

            System.out.println("[8] Testing DDM Spatial Routing (In-Bounds Update)...");
            AttributeHandleValueMap updateIn = new AttributeHandleValueMap();
            updateIn.put(attrCoordinates, "POS:250,250,250".getBytes(StandardCharsets.UTF_8));
            rtiAlfa.updateAttributeValuesSpatial(jetObj, updateIn, 250.0, 250.0, 250.0, new byte[]{0x02});

            Thread.sleep(100);
            rtiBravo.evokeMultipleCallbacks(0.05, 0.3);
            if (bravoReflectionCount.get() != 1) {
                throw new RuntimeException("Test Failed: Bravo did not receive in-bounds spatial reflection! Expected 1, got " + bravoReflectionCount.get());
            }
            System.out.println(" -> DDM Spatial Routing SUCCESS: In-bounds update received via 3D Morton code routing!");

            System.out.println("[9] Testing Ownership Handover (Sprint 6)...");
            boolean alfaOwns = rtiAlfa.isAttributeOwnedByFederate(jetObj, attrCoordinates);
            System.out.println(" -> Alfa ownership before divestiture: " + alfaOwns);
            if (!alfaOwns) {
                throw new RuntimeException("Test Failed: Alfa should own attrCoordinates initially!");
            }

            AttributeHandleSet divestSet = new AttributeHandleSet();
            divestSet.add(attrCoordinates);
            rtiAlfa.unconditionalAttributeOwnershipDivestiture(jetObj, divestSet);
            System.out.println(" -> Alfa divested ownership of attrCoordinates.");

            AttributeHandleSet acquireSet = new AttributeHandleSet();
            acquireSet.add(attrCoordinates);
            rtiBravo.attributeOwnershipAcquisition(jetObj, acquireSet, new byte[]{0x03});
            System.out.println(" -> Bravo acquired ownership of attrCoordinates.");

            boolean bravoOwns = rtiBravo.isAttributeOwnedByFederate(jetObj, attrCoordinates);
            System.out.println(" -> Bravo ownership verification: " + bravoOwns);
            if (!bravoOwns) {
                throw new RuntimeException("Test Failed: Bravo should now own attrCoordinates after handover!");
            }
            System.out.println(" -> Ownership Handover SUCCESS!");

            System.out.println("[10] Resigning and Disconnecting...");
            rtiAlfa.resignFederationExecution(ResignAction.NO_ACTION);
            rtiBravo.resignFederationExecution(ResignAction.NO_ACTION);
            rtiAlfa.destroyFederationExecution(fedName);
            rtiAlfa.disconnect();
            rtiBravo.disconnect();
            System.out.println(" -> Cleanly resigned, destroyed federation, and disconnected.");

            System.out.println("\n***************************************************");
            System.out.println("  [VALHALLA JAVA SDK TEST PASSED: 100% SUCCESSFUL]  ");
            System.out.println("***************************************************\n");

        } catch (Exception e) {
            System.err.println("\n[ERROR] Java Federate Test Failed: " + e.getMessage());
            e.printStackTrace();
            System.exit(1);
        }
    }
}
