# Valhalla Standalone Java Federate Example (IEEE 1516e / HLA-4)

This directory contains a **completely standalone, self-contained IEEE 1516e Java Federate** designed to represent an external simulation developer consuming the Project Valhalla SDK without having the core Valhalla repository cloned.

---

## 1. Architecture Overview

- **Maven Dependency Resolution:** Consumes the official fat JAR `io.valhalla:valhalla-lrc:1.0.0` directly from the sovereign GitHub Maven repository branch (`mvn-repo`).
- **Zero Local Footprint:** Does not inherit from any parent POM; depends purely on the Maven repository declarations.
- **Embedded Dual-Platform Binaries:** The `valhalla-lrc` JAR bundles both `valhalla_lrc.dll` (Windows x64) and `libvalhalla_lrc.so` (Linux x86_64). At runtime, `ValhallaNativeLoader` detects the operating system and extracts the appropriate native library transparently.
- **IEEE 1516e / HLA-4 Homologation:** Implements the official 11-stage compliance test verifying the full simulation lifecycle.

---

## 2. Homologation Verification Stages

When executed, the federate validates all 11 core RTI service areas:

| Stage | Service Area | Verification |
|---|---|---|
| **1** | Connection | Connects to CRC via `RtiFactoryFactory.getRtiFactory().getRtiAmbassador()`. |
| **2** | Federation Execution | Creates federation and joins with dynamic `FederateHandle`. |
| **3** | Declaration Management | Publishes standard MIM `HLAobjectRoot` and `HLAprivilegeToDeleteObject`. |
| **4** | Data Distribution Management (DDM) | Creates 3D spatial region with Morton indexing (`[0, 1000]^3`) and subscribes. |
| **5** | Object Management | Registers object instance (`Valhalla_Asset_Alpha`). |
| **6** | Spatial Object Updates | Sends spatial attribute updates at coordinate `(250.0, 250.0, 250.0)` and evokes callbacks. |
| **7** | Ownership Management | Verifies initial ownership, performs unconditional divestiture, and re-acquires attribute. |
| **8** | Object Deletion | Deletes object instance cleanly with user-supplied tag. |
| **9** | Resignation | Resigns using `DELETE_OBJECTS_THEN_DIVEST`. |
| **10** | Destruction | Destroys federation execution cleanly. |
| **11** | Disconnection | Disconnects from Central RTI Component (CRC). |

---

## 3. How to Run

### Prerequisites
- Java JDK 17+
- Apache Maven 3.8+
- Active Valhalla CRC (optional; `run.bat` / `run.sh` will start a local CRC automatically if `dist/bin/valhalla-crc` is present)

### On Windows
```cmd
:: Run with default CRC (127.0.0.1:52295)
run.bat

:: Or specify a custom/remote CRC address:
run.bat 192.168.1.100:52295
```

### On Linux / macOS
```bash
chmod +x run.sh

# Run with default CRC (127.0.0.1:52295)
./run.sh

# Or specify a custom/remote CRC address:
./run.sh 192.168.1.100:52295
```

---

## 4. Consuming in Your Own Project

To use the Valhalla Java LRC in any standalone Maven project, add the repository and dependency to your `pom.xml`:

```xml
<repositories>
    <repository>
        <id>valhalla-mvn-repo</id>
        <url>https://raw.githubusercontent.com/icemagno/valhalla/mvn-repo/</url>
        <snapshots>
            <enabled>true</enabled>
            <updatePolicy>always</updatePolicy>
        </snapshots>
    </repository>
</repositories>

<dependencies>
    <dependency>
        <groupId>io.valhalla</groupId>
        <artifactId>valhalla-lrc</artifactId>
        <version>1.0.0</version>
    </dependency>
</dependencies>
```
