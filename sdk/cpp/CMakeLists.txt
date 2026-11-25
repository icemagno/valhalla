cmake_minimum_required(VERSION 3.15)
project(valhalla_rti1516e_cpp LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 14)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

include_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}/include
    ${CMAKE_CURRENT_SOURCE_DIR}/../c/include
)

set(SOURCES
    src/ValhallaRTIambassador.cpp
    src/RTIambassadorFactory.cpp
)

add_library(rti1516e STATIC ${SOURCES})

# Target include directories
target_include_directories(rti1516e PUBLIC
    $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
    $<INSTALL_INTERFACE:include>
)

target_include_directories(rti1516e PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/../c/include
)
