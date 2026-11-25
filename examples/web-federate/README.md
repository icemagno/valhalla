# Valhalla HLA-4 Web Federate & Tactical Radar (IEEE 1516-2025 Web Profile)

This example demonstrates the official **HLA-4 Web Profile** running directly inside any web browser (Chrome, Edge, Firefox, Safari) or web runtime, connecting to the **Valhalla CRC** via WebSocket with zero native dependencies!

## Architecture
- **Pure Browser Client:** Zero plugins, zero WebAssembly, zero CGo, zero native binaries.
- **Protocol:** Standard RFC 6455 WebSockets with JSON message payloads.
- **Port:** Defaults to `ws://127.0.0.1:52296/ws`.
- **Integrated Tactical Radar:** Real-time canvas radar showing entities discovered in 3D Morton space and spatial updates.

## How to Run

### 1. Start the Valhalla CRC
```cmd
dist\bin\valhalla-crc.exe -port :52295
```
The CRC will automatically activate both the binary simulation socket on `:52295` and the HLA-4 Web Profile Gateway on `:52296`!

### 2. Open the Web Federate
Simply double click `index.html` or open it in your browser:
```cmd
start examples\web-federate\index.html
```
Or open the built-in CRC dashboard directly at:
```text
http://127.0.0.1:52296/
```

### 3. Run the Homologation Test
Click the **"Run 11-Stage Homologation Test"** button to execute all 11 stages of the standard HLA simulation lifecycle in real-time!
