/**
 * Valhalla HLA-4 Web Profile SDK (IEEE 1516-2025)
 * TypeScript Type Definitions
 */

export declare enum CallbackModel {
    HLA_IMMEDIATE = "HLA_IMMEDIATE",
    HLA_EVOKED = "HLA_EVOKED"
}

export declare enum OrderType {
    RECEIVE = "RECEIVE",
    TIMESTAMP = "TIMESTAMP"
}

export declare enum ResignAction {
    UNCONDITIONALLY_DIVEST_ATTRIBUTES = "UNCONDITIONALLY_DIVEST_ATTRIBUTES",
    DELETE_OBJECTS = "DELETE_OBJECTS",
    CANCEL_PENDING_OWNERSHIP_ACQUISITIONS = "CANCEL_PENDING_OWNERSHIP_ACQUISITIONS",
    DELETE_OBJECTS_THEN_DIVEST = "DELETE_OBJECTS_THEN_DIVEST",
    CANCEL_THEN_DELETE_THEN_DIVEST = "CANCEL_THEN_DELETE_THEN_DIVEST",
    NO_ACTION = "NO_ACTION"
}

export interface FederateAmbassador {
    discoverObjectInstance?(object: number, objectClass: number, objectName: string): void;
    reflectAttributeValues?(object: number, attributes: Record<string, string>, tag?: string): void;
    removeObjectInstance?(object: number, tag?: string): void;
    timeAdvanceGrant?(time: number): void;
    announceSynchronizationPoint?(label: string, tag?: string): void;
    federationSynchronized?(label: string): void;
}

export declare class NullFederateAmbassador implements FederateAmbassador {
    discoverObjectInstance(object: number, objectClass: number, objectName: string): void;
    reflectAttributeValues(object: number, attributes: Record<string, string>, tag?: string): void;
    removeObjectInstance(object: number, tag?: string): void;
    timeAdvanceGrant(time: number): void;
    announceSynchronizationPoint(label: string, tag?: string): void;
    federationSynchronized(label: string): void;
}

export declare class RTIexception extends Error {
    readonly code?: string;
    constructor(message: string, code?: string);
}

export declare class RTIambassador {
    constructor(defaultUrl?: string);
    on(event: string, handler: (...args: any[]) => void): this;
    off(event: string, handler: (...args: any[]) => void): this;
    connect(fedAmbOrUrl?: FederateAmbassador | string, maybeUrl?: string): Promise<void>;
    disconnect(): Promise<void>;
    createFederationExecution(federationName: string, fomModules?: string | string[]): Promise<void>;
    destroyFederationExecution(federationName: string): Promise<void>;
    joinFederationExecution(federateName: string, federationName: string, federateType?: string): Promise<number>;
    resignFederationExecution(resignAction?: ResignAction): Promise<void>;
    getObjectClassHandle(className: string): Promise<number>;
    getAttributeHandle(classHandle: number, attributeName: string): Promise<number>;
    getDimensionHandle(dimensionName: string): Promise<number>;
    publishObjectClassAttributes(classHandle: number, attributeHandles: number[]): Promise<void>;
    subscribeObjectClassAttributes(classHandle: number, attributeHandles: number[]): Promise<void>;
    createRegion(dimensions?: number[]): Promise<number>;
    setMetricBounds(regionHandle: number, minX: number, maxX: number, minY: number, maxY: number, minZ: number, maxZ: number): Promise<void>;
    subscribeObjectClassAttributesWithRegions(classHandle: number, attributeHandles: number[], regionHandle: number): Promise<void>;
    registerObjectInstance(classHandle: number, objectName?: string): Promise<number>;
    deleteObjectInstance(objectHandle: number, tag?: string): Promise<void>;
    updateAttributeValues(objectHandle: number, attributes: Record<number | string, string>, tag?: string): Promise<void>;
    updateAttributeValuesSpatial(objectHandle: number, attributes: Record<number | string, string>, x: number, y: number, z: number, tag?: string): Promise<void>;
    isAttributeOwnedByFederate(objectHandle: number, attributeHandle: number): Promise<boolean>;
    unconditionalAttributeOwnershipDivestiture(objectHandle: number, attributeHandles: number[]): Promise<void>;
    attributeOwnershipAcquisition(objectHandle: number, attributeHandles: number[], tag?: string): Promise<void>;
    timeAdvanceRequest(time: number): Promise<void>;
    evokeCallback(timeoutSeconds?: number): Promise<boolean>;
    getFederateHandle(): number;
}
