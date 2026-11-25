# Valhalla C++ Standalone Example Federate (Official IEEE 1516e / HLA-4)

This example federate demonstrates the **Official Normative C++ Binding (`namespace rti1516e`)** connecting to the sovereign **Valhalla CRC** (`valhalla-crc.exe`) via the zero-overhead C-ABI runtime (`valhalla_lrc.dll` / `libvalhalla_lrc.so`).

## Complete 11-Stage Homologation Test
1. **Discovery & Connection to Central RTI Component (CRC):** Connects to `127.0.0.1:52295` with `CallbackModel::HLA_EVOKED`.
2. **Federation Execution Creation & Join:** Parses and associates `ValhallaAdvanced.xml` (Tactical Aerospace Entities) and joins the federation.
3. **Dynamic FOM Declaration Management:** Dynamically discovers `ObjectClassHandle` for `TacticalEntity` and `AttributeHandle` for `Position`, `Velocity`, and `PayloadStatus` directly from the active FOM. Publishes all attributes.
4. **Dynamic Data Distribution Management (DDM):** Dynamically discovers `DimensionHandle` for `DimensionX`, `DimensionY`, and `DimensionZ`. Creates a 3D Morton-indexed spatial region bounded to `[0.0, 1000.0]^3` and subscribes with region.
5. **Object Instance Registration:** Registers `Valhalla_Tactical_Drone_01`.
6. **3D Spatial Updates & Callback Evocation:** Dispatches spatial updates at `(250.0, 250.0, 250.0)` inside the bounding box and evokes callbacks.
7. **Attribute Ownership Management:** Tests ownership query (`isAttributeOwnedByFederate`), unconditional divestiture, and acquisition of FOM attribute `Position`.
8. **Object Instance Deletion:** Deletes instance cleanly with user-supplied tag.
9. **Clean Resign Action:** Resigns with `ResignAction::DELETE_OBJECTS_THEN_DIVEST`.
10. **Federation Execution Destruction:** Destroys federation.
11. **Disconnect:** Closes session cleanly.

## Running the Example

### 1. Start the Valhalla CRC
In a separate terminal:
```cmd
dist\bin\valhalla-crc.exe -port :52295
```

### 2. Run the C++ Federate
```cmd
cd examples\cpp-federate
run.bat
```
Or with custom CRC address:
```cmd
run.bat 127.0.0.1:52295 MyFederation MyCppFederate
```
