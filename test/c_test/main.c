#include <stdio.h>
#include <stdint.h>
#include <stdbool.h>
#include "../../pkg/cabi/valhalla_lrc.h"

int main(void) {
    printf("[C-TEST] Creating Ambassador...\n");
    ValhallaAmbassador amb = valhalla_create_ambassador();
    if (!amb) {
        printf("[C-TEST] FAIL: valhalla_create_ambassador returned NULL\n");
        return 1;
    }

    printf("[C-TEST] Disconnecting idle ambassador...\n");
    int32_t rc = valhalla_disconnect(amb);
    printf("[C-TEST] valhalla_disconnect returned %d\n", rc);
    if (rc != VALHALLA_OK && rc != VALHALLA_ERR_NOT_CONNECTED) {
        printf("[C-TEST] FAIL: unexpected disconnect status %d\n", rc);
        valhalla_destroy_ambassador(amb);
        return 1;
    }

    printf("[C-TEST] Destroying Ambassador...\n");
    valhalla_destroy_ambassador(amb);
    printf("[C-TEST] SUCCESS: C-ABI Ambassador lifecycle verified!\n");
    return 0;
}
