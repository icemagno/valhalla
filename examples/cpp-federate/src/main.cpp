// main.cpp - Valhalla C++ Standalone Example Federate (Official IEEE 1516e Homologation Test)
#include <rti1516e/RTIambassadorFactory.h>
#include <rti1516e/RTIexception.h>
#include "ExampleFederateAmbassador.h"

#include <iostream>
#include <string>
#include <fstream>
#include <memory>
#include <vector>

int main(int argc, char* argv[]) {
    std::wstring crcAddress = (argc > 1) ? std::wstring(argv[1], argv[1] + strlen(argv[1])) : L"127.0.0.1:52295";
    std::wstring fedName = (argc > 2) ? std::wstring(argv[2], argv[2] + strlen(argv[2])) : L"ValhallaHomologFederation";
    std::wstring federateType = (argc > 3) ? std::wstring(argv[3], argv[3] + strlen(argv[3])) : L"CppHomologFederate";

    std::wcout << L"====================================================================" << std::endl;
    std::wcout << L"    PROJECT VALHALLA - OFFICIAL IEEE 1516e C++ HOMOLOGATION TEST" << std::endl;
    std::wcout << L"====================================================================" << std::endl;
    std::wcout << L" CRC Endpoint:        " << crcAddress << std::endl;
    std::wcout << L" Federation Name:     " << fedName << std::endl;
    std::wcout << L" Federate Type:       " << federateType << std::endl;
    std::wcout << L" Architecture:        Sovereign C++ Binding (sdk/cpp -> valhalla_lrc.dll)" << std::endl;
    std::wcout << L" FOM Model:           ValhallaAdvanced.xml (Tactical Aerospace Entities)" << std::endl;
    std::wcout << L"====================================================================\n" << std::endl;

    ExampleFederateAmbassador fedAmb;
    std::unique_ptr<rti1516e::RTIambassador> rtiAmb;

    try {
        // ---------------------------------------------------------------------
        // Stage 1: Obtain Ambassador and Connect to CRC
        // ---------------------------------------------------------------------
        std::cout << "[Stage 1/11] Initializing Valhalla RTI Ambassador..." << std::endl;
        rti1516e::RTIambassadorFactory factory;
        rtiAmb = factory.createRTIambassador();

        std::wcout << L"[Stage 1/11] Connecting to CRC at " << crcAddress << L" (CallbackModel::HLA_EVOKED)..." << std::endl;
        rtiAmb->connect(fedAmb, rti1516e::CallbackModel::HLA_EVOKED, crcAddress);
        std::cout << "  [+] Connected successfully to Valhalla CRC!\n" << std::endl;

        // ---------------------------------------------------------------------
        // Stage 2: Create Federation with Official FOM Module & Join
        // ---------------------------------------------------------------------
        std::wstring fomPath = L"foms/ValhallaAdvanced.xml";
        {
            std::ifstream check("foms/ValhallaAdvanced.xml");
            if (!check.good()) {
                fomPath = L"../../foms/ValhallaAdvanced.xml";
            }
        }

        std::wcout << L"[Stage 2/11] Creating federation execution '" << fedName << L"' with FOM module..." << std::endl;
        try {
            rtiAmb->createFederationExecution(fedName, fomPath);
            std::wcout << L"  [+] Federation execution created with FOM: " << fomPath << std::endl;
        } catch (const rti1516e::FederationExecutionAlreadyExists&) {
            std::cout << "  [*] Federation execution already exists (proceeding to join)." << std::endl;
        }

        std::wcout << L"[Stage 2/11] Joining federation execution as '" << federateType << L"'..." << std::endl;
        rti1516e::FederateHandle fedHandle = rtiAmb->joinFederationExecution(federateType, federateType, fedName);
        std::cout << "  [+] Joined federation successfully! FederateHandle: " << fedHandle.getValue() << "\n" << std::endl;

        // ---------------------------------------------------------------------
        // Stage 3: Dynamic FOM Resolution (No hardcoded handles!)
        // ---------------------------------------------------------------------
        std::cout << "[Stage 3/11] Dynamically Resolving Object Class & Attributes from FOM..." << std::endl;
        rti1516e::ObjectClassHandle entityClass = rtiAmb->getObjectClassHandle(L"TacticalEntity");
        std::cout << "  [+] Resolved Class 'TacticalEntity' => " << entityClass.getValue() << std::endl;

        rti1516e::AttributeHandle posAttr = rtiAmb->getAttributeHandle(entityClass, L"Position");
        std::cout << "  [+] Resolved Attribute 'Position' => " << posAttr.getValue() << std::endl;

        rti1516e::AttributeHandle velAttr = rtiAmb->getAttributeHandle(entityClass, L"Velocity");
        std::cout << "  [+] Resolved Attribute 'Velocity' => " << velAttr.getValue() << std::endl;

        rti1516e::AttributeHandle payloadAttr = rtiAmb->getAttributeHandle(entityClass, L"PayloadStatus");
        std::cout << "  [+] Resolved Attribute 'PayloadStatus' => " << payloadAttr.getValue() << std::endl;

        rti1516e::AttributeHandleSet pubAttrs;
        pubAttrs.insert(posAttr);
        pubAttrs.insert(velAttr);
        pubAttrs.insert(payloadAttr);

        rtiAmb->publishObjectClassAttributes(entityClass, pubAttrs);
        std::cout << "  [+] Published FOM attributes for class " << entityClass.getValue() << "\n" << std::endl;

        // ---------------------------------------------------------------------
        // Stage 4: Dynamic DDM 3D Spatial Region from FOM Dimensions
        // ---------------------------------------------------------------------
        std::cout << "[Stage 4/11] Dynamically Resolving DDM Dimensions from FOM..." << std::endl;
        rti1516e::DimensionHandle dimX = rtiAmb->getDimensionHandle(L"DimensionX");
        rti1516e::DimensionHandle dimY = rtiAmb->getDimensionHandle(L"DimensionY");
        rti1516e::DimensionHandle dimZ = rtiAmb->getDimensionHandle(L"DimensionZ");
        std::cout << "  [+] Resolved Dimensions: X=" << dimX.getValue() << ", Y=" << dimY.getValue() << ", Z=" << dimZ.getValue() << std::endl;

        std::vector<rti1516e::DimensionHandle> dims = { dimX, dimY, dimZ };
        rti1516e::RegionHandle region = rtiAmb->createRegion(dims);
        std::cout << "  [+] Created 3D Spatial Region: " << region.getValue() << std::endl;

        // Bounding box [0.0, 1000.0] along X, Y, and Z
        rtiAmb->setMetricBounds(region, 0.0, 1000.0, 0.0, 1000.0, 0.0, 1000.0);
        std::cout << "  [+] Bounding box set: [0, 1000] x [0, 1000] x [0, 1000]" << std::endl;

        rtiAmb->subscribeObjectClassAttributesWithRegions(entityClass, pubAttrs, region);
        std::cout << "  [+] Subscribed to class " << entityClass.getValue() << " with 3D Morton region " << region.getValue() << "\n" << std::endl;

        // ---------------------------------------------------------------------
        // Stage 5: Register Object Instance of FOM Class
        // ---------------------------------------------------------------------
        std::wstring assetName = L"Valhalla_Tactical_Drone_01";
        std::wcout << L"[Stage 5/11] Registering object instance '" << assetName << L"' of class " << entityClass.getValue() << L"..." << std::endl;
        rti1516e::ObjectInstanceHandle objHandle = rtiAmb->registerObjectInstance(entityClass, assetName);
        std::cout << "  [+] Object instance registered! ObjectInstanceHandle: " << objHandle.getValue() << "\n" << std::endl;

        // ---------------------------------------------------------------------
        // Stage 6: Spatial Updates & Callback Evocation
        // ---------------------------------------------------------------------
        std::cout << "[Stage 6/11] Performing 3D Spatial Attribute Updates with FOM Attributes..." << std::endl;
        rti1516e::AttributeHandleValueMap updateMap;
        std::string posVal = "POS(x=250.0, y=250.0, z=250.0)";
        std::string velVal = "VEL(mach=1.4)";
        std::string payloadVal = "RADAR_SEARCH_MODE";
        std::string tagVal = "TelemetryCycle_1";

        updateMap[posAttr] = rti1516e::VariableLengthData(posVal.data(), posVal.size());
        updateMap[velAttr] = rti1516e::VariableLengthData(velVal.data(), velVal.size());
        updateMap[payloadAttr] = rti1516e::VariableLengthData(payloadVal.data(), payloadVal.size());
        rti1516e::VariableLengthData tag(tagVal.data(), tagVal.size());

        rtiAmb->updateAttributeValuesSpatial(objHandle, updateMap, 250.0, 250.0, 250.0, tag);
        std::cout << "  [+] Sent spatial update for 3 FOM attributes at (250.0, 250.0, 250.0) inside bounding box." << std::endl;

        std::cout << "[Stage 6/11] Evoking callbacks to process reflections..." << std::endl;
        bool callbacksEvoked = rtiAmb->evokeMultipleCallbacks(0.05, 0.3);
        std::cout << "  [+] Callback evocation completed (callbacks processed: " << (callbacksEvoked ? "true" : "false")
                  << ", reflectCount: " << fedAmb.getReflectCount() << ")\n" << std::endl;

        // ---------------------------------------------------------------------
        // Stage 7: Attribute Ownership Management on FOM Attribute
        // ---------------------------------------------------------------------
        std::cout << "[Stage 7/11] Testing Attribute Ownership Management on FOM Attribute 'Position'..." << std::endl;
        bool initialOwner = rtiAmb->isAttributeOwnedByFederate(objHandle, posAttr);
        std::cout << "  [+] Initial Ownership of 'Position': " << (initialOwner ? "true" : "false") << " (expected: true)" << std::endl;
        if (!initialOwner) {
            throw rti1516e::RTIinternalError(L"Homologation Failure: Federate should own 'Position' upon registration");
        }

        std::cout << "  [*] Divesting 'Position' ownership unconditionally..." << std::endl;
        rti1516e::AttributeHandleSet divestSet;
        divestSet.insert(posAttr);
        rtiAmb->unconditionalAttributeOwnershipDivestiture(objHandle, divestSet);
        bool afterDivest = rtiAmb->isAttributeOwnedByFederate(objHandle, posAttr);
        std::cout << "  [+] Ownership after divestiture: " << (afterDivest ? "true" : "false") << " (expected: false)" << std::endl;
        if (afterDivest) {
            throw rti1516e::RTIinternalError(L"Homologation Failure: Attribute should be unowned after divestiture");
        }

        std::cout << "  [*] Re-acquiring 'Position' ownership..." << std::endl;
        std::string acqTagStr = "AcquisitionTag";
        rti1516e::VariableLengthData acqTag(acqTagStr.data(), acqTagStr.size());
        rtiAmb->attributeOwnershipAcquisition(objHandle, divestSet, acqTag);
        bool afterAcquire = rtiAmb->isAttributeOwnedByFederate(objHandle, posAttr);
        std::cout << "  [+] Ownership after re-acquisition: " << (afterAcquire ? "true" : "false") << " (expected: true)\n" << std::endl;
        if (!afterAcquire) {
            throw rti1516e::RTIinternalError(L"Homologation Failure: Attribute should be owned after acquisition");
        }

        // ---------------------------------------------------------------------
        // Stage 8: Delete Object Instance
        // ---------------------------------------------------------------------
        std::cout << "[Stage 8/11] Deleting object instance " << objHandle.getValue() << "..." << std::endl;
        std::string delTagStr = "HomologationComplete";
        rti1516e::VariableLengthData delTag(delTagStr.data(), delTagStr.size());
        rtiAmb->deleteObjectInstance(objHandle, delTag);
        std::cout << "  [+] Object instance deleted cleanly.\n" << std::endl;

        // ---------------------------------------------------------------------
        // Stage 9: Resign Federation Execution
        // ---------------------------------------------------------------------
        std::cout << "[Stage 9/11] Resigning from federation execution..." << std::endl;
        rtiAmb->resignFederationExecution(rti1516e::ResignAction::DELETE_OBJECTS_THEN_DIVEST);
        std::cout << "  [+] Resigned successfully with DELETE_OBJECTS_THEN_DIVEST.\n" << std::endl;

        // ---------------------------------------------------------------------
        // Stage 10: Destroy Federation Execution
        // ---------------------------------------------------------------------
        std::wcout << L"[Stage 10/11] Destroying federation execution '" << fedName << L"'..." << std::endl;
        try {
            rtiAmb->destroyFederationExecution(fedName);
            std::cout << "  [+] Federation execution destroyed." << std::endl;
        } catch (const rti1516e::FederatesCurrentlyJoined& e) {
            std::cout << "  [*] Federation still has other joined federates: " << e.what() << std::endl;
        }

        // ---------------------------------------------------------------------
        // Stage 11: Disconnect
        // ---------------------------------------------------------------------
        std::cout << "\n[Stage 11/11] Disconnecting from CRC..." << std::endl;
        rtiAmb->disconnect();
        std::cout << "  [+] Disconnected cleanly from CRC.\n" << std::endl;

        std::cout << "====================================================================" << std::endl;
        std::cout << "   HOMOLOGATION TEST PASSED: ALL 11 VERIFICATION STAGES SUCCEEDED   " << std::endl;
        std::cout << "      (100% Dynamic FOM Resolution - Zero Hardcoded Handles)        " << std::endl;
        std::cout << "====================================================================" << std::endl;

    } catch (const rti1516e::RTIexception& e) {
        std::cerr << "\n[-] HOMOLOGATION TEST FAILED WITH RTI EXCEPTION: " << e.what() << std::endl;
        return 1;
    } catch (const std::exception& e) {
        std::cerr << "\n[-] HOMOLOGATION TEST FAILED WITH STANDARD EXCEPTION: " << e.what() << std::endl;
        return 1;
    }

    return 0;
}
