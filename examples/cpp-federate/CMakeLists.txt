cmake_minimum_required(VERSION 3.15)
project(valhalla_cpp_federate LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 14)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

include_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}/src
    ${CMAKE_CURRENT_SOURCE_DIR}/../../dist/include
)

link_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}/../../dist/lib
    ${CMAKE_CURRENT_SOURCE_DIR}/../../dist/bin
)

add_executable(cpp_federate src/main.cpp)

target_link_libraries(cpp_federate
    PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/../../dist/lib/librti1516e.a
    ${CMAKE_CURRENT_SOURCE_DIR}/../../dist/bin/valhalla_lrc.dll
)
