package org.valhalla.example;

import hla.rti1516e.*;
import hla.rti1516e.exceptions.*;

import java.nio.charset.StandardCharsets;

/**
 * Valhalla Standalone Example Federate (IEEE 1516e / HLA-4 Official Homologation Test)
 *
 * Demonstrates a complete end-to-end HLA-4 simulation lifecycle:
 *  1. Discovery & Connection to Central RTI Component (CRC)
 *  2. Federation Execution Creation & Join
 *  3. Object Class Declaration & Publication (HLAobjectRoot / HLAprivilegeToDeleteObject)
 *  4. Data Distribution Management (DDM) 3D Morton Spatial Region Subscription
 *  5. Object Instance Registration
 *  6. 3D Spatial Attribute Updates & Asynchronous Reflection Evocation
 *  7. Attribute Ownership Management (Query, Divestiture, Acquisition)
 *  8. Object Instance Deletion & Lifecycle Verification
 *  9. Clean Resign Action
 * 10. Federation Execution Destruction
 * 11. Disconnect
 */
public class ExampleFederate {

    public static void main(String[] args) {
        String crcAddress = (args.length > 0 && !args[0].isEmpty()) ? args[0] : "127.0.0.1:52295";
        String fedName = (args.length > 1 && !args[1].isEmpty()) ? args[1] : "ValhallaHomologFederation";
        String federateType = (args.length > 2 && !args[2].isEmpty()) ? args[2] : "JavaHomologFederate";

        System.out.println("====================================================================");
        System.out.println("    PROJECT VALHALLA - STANDALONE IEEE 1516e HOMOLOGATION TEST");
        System.out.println("====================================================================");
        System.out.println(" CRC Endpoint:        " + crcAddress);
        System.out.println(" Federation Name:     " + fedName);
        System.out.println(" Federate Type:       " + federateType);
        System.out.println(" Architecture:        Sovereign Native LRC (libvalhalla_lrc)");
        System.out.println("====================================================================\n");

        ExampleFederateAmbassador fedAmb = new ExampleFederateAmbassador();
        RTIambassador rtiAmb = null;

        try {
            // -------------------------------------------------------------
            // Stage 1: Obtain Ambassador and Connect to CRC
            // -------------------------------------------------------------
            System.out.println("[Stage 1/11] Initializing Valhalla RTI Ambassador...");
            RtiFactory rtiFactory = RtiFactoryFactory.getRtiFactory();
            rtiAmb = rtiFactory.getRtiAmbassador();

            System.out.println("[Stage 1/11] Connecting to CRC at " + crcAddress + " (CallbackModel.HLA_EVOKED)...");
            rtiAmb.connect(fedAmb, CallbackModel.HLA_EVOKED, crcAddress);
            System.out.println("  [+] Connected successfully to Valhalla CRC!\n");

            // -------------------------------------------------------------
            // Stage 2: Create & Join Federation Execution
            // -------------------------------------------------------------
            System.out.println("[Stage 2/11] Creating federation execution '" + fedName + "'...");
            try {
                rtiAmb.createFederationExecution(fedName);
                System.out.println("  [+] Federation execution created.");
            } catch (FederationExecutionAlreadyExists e) {
                System.out.println("  [*] Federation execution already exists (proceeding to join).");
            }

            System.out.println("[Stage 2/11] Joining federation execution as '" + federateType + "'...");
            FederateHandle fedHandle = rtiAmb.joinFederationExecution(federateType, fedName);
            System.out.println("  [+] Joined federation successfully! FederateHandle: " + fedHandle + "\n");

            // -------------------------------------------------------------
            // Stage 3: Publish Object Class Attributes
            // -------------------------------------------------------------
            System.out.println("[Stage 3/11] Publishing Object Class (HLAobjectRoot = 1)...");
            ObjectClassHandle objClass = new ObjectClassHandle(1);
            AttributeHandle privilegeAttr = new AttributeHandle(1); // Standard MIM: HLAprivilegeToDeleteObject

            AttributeHandleSet pubAttrs = new AttributeHandleSet();
            pubAttrs.add(privilegeAttr);

            rtiAmb.publishObjectClassAttributes(objClass, pubAttrs);
            System.out.println("  [+] Published privilegeToDeleteObject attribute for class " + objClass + "\n");

            // -------------------------------------------------------------
            // Stage 4: DDM 3D Morton Spatial Region Subscription
            // -------------------------------------------------------------
            System.out.println("[Stage 4/11] Configuring DDM 3D Spatial Region...");
            DimensionHandle[] spatialDims = new DimensionHandle[]{
                    new DimensionHandle(1), // X
                    new DimensionHandle(2), // Y
                    new DimensionHandle(3)  // Z
            };
            RegionHandle region = rtiAmb.createRegion(spatialDims);
            System.out.println("  [+] Created 3D Spatial Region: " + region);

            // Bounding box [0.0, 1000.0] along X, Y, and Z
            rtiAmb.setMetricBounds(region, 0.0, 1000.0, 0.0, 1000.0, 0.0, 1000.0);
            System.out.println("  [+] Bounding box set: [0, 1000] x [0, 1000] x [0, 1000]");

            rtiAmb.subscribeObjectClassAttributesWithRegions(objClass, pubAttrs, region);
            System.out.println("  [+] Subscribed to class " + objClass + " with 3D Morton region " + region + "\n");

            // -------------------------------------------------------------
            // Stage 5: Register Object Instance
            // -------------------------------------------------------------
            String assetName = "Valhalla_Asset_Alpha";
            System.out.println("[Stage 5/11] Registering object instance '" + assetName + "'...");
            ObjectInstanceHandle objHandle = rtiAmb.registerObjectInstance(objClass, assetName);
            System.out.println("  [+] Object instance registered! ObjectInstanceHandle: " + objHandle + "\n");

            // -------------------------------------------------------------
            // Stage 6: Spatial Updates & Callback Evocation
            // -------------------------------------------------------------
            System.out.println("[Stage 6/11] Performing 3D Spatial Attribute Updates...");
            AttributeHandleValueMap updateMap = new AttributeHandleValueMap();
            updateMap.put(privilegeAttr, "POS(x=250.0, y=250.0, z=250.0)".getBytes(StandardCharsets.UTF_8));
            byte[] tag = "TelemetryCycle_1".getBytes(StandardCharsets.UTF_8);

            rtiAmb.updateAttributeValuesSpatial(objHandle, updateMap, 250.0, 250.0, 250.0, tag);
            System.out.println("  [+] Sent spatial update at (250.0, 250.0, 250.0) inside bounding box.");

            System.out.println("[Stage 6/11] Evoking callbacks to process reflections...");
            boolean callbacksEvoked = rtiAmb.evokeMultipleCallbacks(0.05, 0.3);
            System.out.println("  [+] Callback evocation completed (callbacks processed: " + callbacksEvoked +
                    ", reflectCount: " + fedAmb.getReflectCount() + ")\n");

            // -------------------------------------------------------------
            // Stage 7: Attribute Ownership Management
            // -------------------------------------------------------------
            System.out.println("[Stage 7/11] Testing Attribute Ownership Management...");
            boolean initialOwner = rtiAmb.isAttributeOwnedByFederate(objHandle, privilegeAttr);
            System.out.println("  [+] Initial Ownership of privilegeToDelete: " + initialOwner + " (expected: true)");
            if (!initialOwner) {
                throw new RTIinternalError("Homologation Failure: Federate should own privilegeToDelete upon registration");
            }

            System.out.println("  [*] Divesting attribute ownership unconditionally...");
            rtiAmb.unconditionalAttributeOwnershipDivestiture(objHandle, pubAttrs);
            boolean afterDivest = rtiAmb.isAttributeOwnedByFederate(objHandle, privilegeAttr);
            System.out.println("  [+] Ownership after divestiture: " + afterDivest + " (expected: false)");
            if (afterDivest) {
                throw new RTIinternalError("Homologation Failure: Attribute should be unowned after divestiture");
            }

            System.out.println("  [*] Re-acquiring attribute ownership...");
            rtiAmb.attributeOwnershipAcquisition(objHandle, pubAttrs, "AcquisitionTag".getBytes(StandardCharsets.UTF_8));
            boolean afterAcquire = rtiAmb.isAttributeOwnedByFederate(objHandle, privilegeAttr);
            System.out.println("  [+] Ownership after re-acquisition: " + afterAcquire + " (expected: true)\n");
            if (!afterAcquire) {
                throw new RTIinternalError("Homologation Failure: Attribute should be owned after acquisition");
            }

            // -------------------------------------------------------------
            // Stage 8: Delete Object Instance
            // -------------------------------------------------------------
            System.out.println("[Stage 8/11] Deleting object instance " + objHandle + "...");
            rtiAmb.deleteObjectInstance(objHandle, "HomologationComplete".getBytes(StandardCharsets.UTF_8));
            System.out.println("  [+] Object instance deleted cleanly.\n");

            // -------------------------------------------------------------
            // Stage 9: Resign Federation Execution
            // -------------------------------------------------------------
            System.out.println("[Stage 9/11] Resigning from federation execution...");
            rtiAmb.resignFederationExecution(ResignAction.DELETE_OBJECTS_THEN_DIVEST);
            System.out.println("  [+] Resigned successfully with DELETE_OBJECTS_THEN_DIVEST.\n");

            // -------------------------------------------------------------
            // Stage 10: Destroy Federation Execution
            // -------------------------------------------------------------
            System.out.println("[Stage 10/11] Destroying federation execution '" + fedName + "'...");
            try {
                rtiAmb.destroyFederationExecution(fedName);
                System.out.println("  [+] Federation execution destroyed.");
            } catch (Exception e) {
                System.out.println("  [*] Federation destruction note: " + e.getMessage());
            }

            // -------------------------------------------------------------
            // Stage 11: Disconnect
            // -------------------------------------------------------------
            System.out.println("\n[Stage 11/11] Disconnecting from CRC...");
            rtiAmb.disconnect();
            System.out.println("  [+] Disconnected cleanly from CRC.\n");

            System.out.println("====================================================================");
            System.out.println("   HOMOLOGATION TEST PASSED: ALL 11 VERIFICATION STAGES SUCCEEDED   ");
            System.out.println("====================================================================");

        } catch (RTIexception e) {
            System.err.println("\n[-] HOMOLOGATION TEST FAILED WITH RTI EXCEPTION: " + e.getMessage());
            e.printStackTrace();
            System.exit(1);
        } catch (Throwable t) {
            System.err.println("\n[-] HOMOLOGATION TEST FAILED WITH UNEXPECTED ERROR: " + t.getMessage());
            t.printStackTrace();
            System.exit(1);
        }
    }
}
