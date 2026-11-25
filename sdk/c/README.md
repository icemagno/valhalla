# Valhalla Native C / C++ SDK

This directory provides the C/C++ client SDK for Project Valhalla. It exposes the standard C-ABI interface ([`include/valhalla_lrc.h`](include/valhalla_lrc.h)) and provides dynamic linking against the native shared libraries compiled into `dist/bin/`.

## Directory Structure
```
sdk/c/
├── include/
│   └── valhalla_lrc.h       # C-ABI header definitions
└── lib/
    ├── valhalla_lrc.dll     # Windows 64-bit dynamic library (copied from dist/bin/)
    └── libvalhalla_lrc.so   # Linux 64-bit shared library (copied from dist/bin/)
```

## Compilation and Linking Example

### Windows (GCC / MinGW-W64)
```cmd
gcc -I sdk/c/include main.c -L dist/bin -lvalhalla_lrc -o my_federate.exe
my_federate.exe
```

### Linux (GCC / Clang)
```bash
gcc -I sdk/c/include main.c -L dist/bin -lvalhalla_lrc -o my_federate
LD_LIBRARY_PATH=dist/bin ./my_federate
```

### CMake Integration
```cmake
cmake_minimum_required(VERSION 3.16)
project(MyFederate C)

include_directories(${CMAKE_CURRENT_SOURCE_DIR}/sdk/c/include)
link_directories(${CMAKE_CURRENT_SOURCE_DIR}/dist/bin)

add_executable(my_federate main.c)
target_link_libraries(my_federate valhalla_lrc)
```
