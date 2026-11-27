#include <stdio.h>
#include <stdint.h>
#include <stdbool.h>
#include <string.h>
#include "../../include/valhalla_lrc.h"

static int g_callbacks_invoked = 0;

static void on_discover(int32_t obj, int32_t cls, const char* name, void* userData) {
    (void)userData;
    g_callbacks_invoked++;
    printf("[C-TEST] Callback DiscoverObject: handle=%d class=%d name=%s\n", obj, cls, name ? name : "");
}

int main(int argc, char** argv) {
    const char* addr = (argc > 1 && argv[1] && argv[1][0] != '\0') ? argv[1] : "127.0.0.1:52295";
    const char* fedName = "ValhallaCTestFederation";
    const char* nodeName = "CTestFederate";

    printf("[C-TEST] 1. Creating Ambassador...\n");
    ValhallaAmbassador amb = valhalla_create_ambassador();
    if (!amb) {
        fprintf(stderr, "[C-TEST] FAIL: valhalla_create_ambassador returned NULL\n");
        return 1;
    }

    ValhallaCallbacks cbs;
    memset(&cbs, 0, sizeof(cbs));
    cbs.onDiscoverObject = on_discover;
    int32_t rc = valhalla_set_callbacks(amb, cbs);
    if (rc != VALHALLA_OK) {
        fprintf(stderr, "[C-TEST] FAIL: valhalla_set_callbacks returned %d\n", rc);
        valhalla_destroy_ambassador(amb);
        return 1;
    }

    printf("[C-TEST] 2. Connecting to CRC at %s...\n", addr);
    rc = valhalla_connect(amb, (char*)addr);
    if (rc != VALHALLA_OK) {
        fprintf(stderr, "[C-TEST] FAIL: valhalla_connect(%s) returned %d\n", addr, rc);
        valhalla_destroy_ambassador(amb);
        return 1;
    }

    printf("[C-TEST] 3. Creating & Joining Federation '%s'...\n", fedName);
    rc = valhalla_create_federation(amb, (char*)fedName);
    if (rc != VALHALLA_OK && rc != VALHALLA_ERR_FEDERATION_EXISTS) {
        fprintf(stderr, "[C-TEST] FAIL: valhalla_create_federation returned %d\n", rc);
        valhalla_disconnect(amb);
        valhalla_destroy_ambassador(amb);
        return 1;
    }

    int32_t fedHandle = 0;
    rc = valhalla_join_federation(amb, (char*)nodeName, (char*)fedName, &fedHandle);
    if (rc != VALHALLA_OK || fedHandle <= 0) {
        fprintf(stderr, "[C-TEST] FAIL: valhalla_join_federation returned %d (handle=%d)\n", rc, fedHandle);
        valhalla_destroy_federation(amb, (char*)fedName);
        valhalla_disconnect(amb);
        valhalla_destroy_ambassador(amb);
        return 1;
    }

    printf("[C-TEST] 4. Evoking callbacks & Resigning (handle=%d)...\n", fedHandle);
    bool evoked = false;
    rc = valhalla_evoke_callback(amb, 0.05, &evoked);
    if (rc != VALHALLA_OK) {
        fprintf(stderr, "[C-TEST] FAIL: valhalla_evoke_callback returned %d\n", rc);
        valhalla_resign_federation(amb);
        valhalla_destroy_federation(amb, (char*)fedName);
        valhalla_disconnect(amb);
        valhalla_destroy_ambassador(amb);
        return 1;
    }

    rc = valhalla_resign_federation(amb);
    if (rc != VALHALLA_OK) {
        fprintf(stderr, "[C-TEST] FAIL: valhalla_resign_federation returned %d\n", rc);
        valhalla_disconnect(amb);
        valhalla_destroy_ambassador(amb);
        return 1;
    }

    rc = valhalla_destroy_federation(amb, (char*)fedName);
    if (rc != VALHALLA_OK && rc != VALHALLA_ERR_FEDERATES_JOINED) {
        fprintf(stderr, "[C-TEST] FAIL: valhalla_destroy_federation returned %d\n", rc);
        valhalla_disconnect(amb);
        valhalla_destroy_ambassador(amb);
        return 1;
    }

    rc = valhalla_disconnect(amb);
    if (rc != VALHALLA_OK) {
        fprintf(stderr, "[C-TEST] FAIL: valhalla_disconnect returned %d\n", rc);
        valhalla_destroy_ambassador(amb);
        return 1;
    }

    valhalla_destroy_ambassador(amb);
    printf("[C-TEST] SUCCESS: Full connected C-ABI lifecycle verified (callbacks=%d)!\n", g_callbacks_invoked);
    return 0;
}
