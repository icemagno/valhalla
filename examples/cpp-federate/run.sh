#!/bin/bash
set -e
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

if [ ! -f cpp_federate ]; then
    echo "[*] Building cpp_federate..."
    g++ -std=c++14 -Wno-deprecated src/main.cpp -Isrc -I../../dist/include ../../dist/lib/librti1516e.a -L../../dist/bin -lvalhalla_lrc -o cpp_federate
fi

export LD_LIBRARY_PATH="../../dist/bin:$LD_LIBRARY_PATH"

echo "[*] Launching C++ Homologation Federate..."
./cpp_federate "$@"
