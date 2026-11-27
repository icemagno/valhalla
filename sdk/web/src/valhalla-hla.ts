/**
 * Valhalla HLA-4 Web Profile SDK (IEEE 1516-2025)
 * Pure Framework-Agnostic TypeScript/JavaScript Client (Browser, Node.js, React, Vue, Angular, Svelte)
 * Zero external dependencies.
 */

export enum CallbackModel {
    HLA_IMMEDIATE = 'HLA_IMMEDIATE',
    HLA_EVOKED = 'HLA_EVOKED'
}

export enum OrderType {
    RECEIVE = 'RECEIVE',
    TIMESTAMP = 'TIMESTAMP'
}

export enum ResignAction {
    UNCONDITIONALLY_DIVEST_ATTRIBUTES = 'UNCONDITIONALLY_DIVEST_ATTRIBUTES',
    DELETE_OBJECTS = 'DELETE_OBJECTS',
    CANCEL_PENDING_OWNERSHIP_ACQUISITIONS = 'CANCEL_PENDING_OWNERSHIP_ACQUISITIONS',
    DELETE_OBJECTS_THEN_DIVEST = 'DELETE_OBJECTS_THEN_DIVEST',
    CANCEL_THEN_DELETE_THEN_DIVEST = 'CANCEL_THEN_DELETE_THEN_DIVEST',
    NO_ACTION = 'NO_ACTION'
}

export interface FederateAmbassador {
    discoverObjectInstance?(object: number, objectClass: number, objectName: string): void;
    reflectAttributeValues?(object: number, attributes: Record<string, string>, tag?: string): void;
    removeObjectInstance?(object: number, tag?: string): void;
    timeAdvanceGrant?(time: number): void;
    announceSynchronizationPoint?(label: string, tag?: string): void;
    federationSynchronized?(label: string): void;
}

export class NullFederateAmbassador implements FederateAmbassador {
    discoverObjectInstance(object: number, objectClass: number, objectName: string): void {}
    reflectAttributeValues(object: number, attributes: Record<string, string>, tag?: string): void {}
    removeObjectInstance(object: number, tag?: string): void {}
    timeAdvanceGrant(time: number): void {}
    announceSynchronizationPoint(label: string, tag?: string): void {}
    federationSynchronized(label: string): void {}
}

export class RTIexception extends Error {
    constructor(message: string, public readonly code?: string) {
        super(message);
        this.name = code || 'RTIexception';
    }
}

export class RTIambassador {
    private ws: any = null;
    private reqId = 0;
    private pendingRequests = new Map<number, { resolve: (res: any) => void; reject: (err: any) => void }>();
    private listeners = new Map<string, Array<(...args: any[]) => void>>();
    private fedAmb?: FederateAmbassador;
    private federateHandle = 0;
    private connected = false;

    constructor(private defaultUrl: string = 'ws://127.0.0.1:52296/ws') {}

    public on(event: string, handler: (...args: any[]) => void): this {
        if (!this.listeners.has(event)) {
            this.listeners.set(event, []);
        }
        this.listeners.get(event)!.push(handler);
        return this;
    }

    public off(event: string, handler: (...args: any[]) => void): this {
        const list = this.listeners.get(event);
        if (list) {
            this.listeners.set(event, list.filter(h => h !== handler));
        }
        return this;
    }

    private emit(event: string, ...args: any[]): void {
        const list = this.listeners.get(event);
        if (list) {
            for (const h of list) {
                try { h(...args); } catch (e) { console.error(`[RTIambassador] Error in listener for ${event}:`, e); }
            }
        }
    }

    public async connect(fedAmbOrUrl?: FederateAmbassador | string, maybeUrl?: string): Promise<void> {
        if (this.connected) return;

        let url = this.defaultUrl;
        if (typeof fedAmbOrUrl === 'string') {
            url = fedAmbOrUrl;
        } else if (fedAmbOrUrl) {
            this.fedAmb = fedAmbOrUrl;
            if (maybeUrl) url = maybeUrl;
        }

        // Support browser native WebSocket or Node.js ws
        let WSClass: any;
        if (typeof WebSocket !== 'undefined') {
            WSClass = WebSocket;
        } else if (typeof globalThis !== 'undefined' && (globalThis as any).WebSocket) {
            WSClass = (globalThis as any).WebSocket;
        } else {
            try {
                WSClass = require('ws');
            } catch (e) {
                throw new RTIexception("WebSocket is not available in current environment. Install 'ws' for Node.js.");
            }
        }

        return new Promise((resolve, reject) => {
            try {
                this.ws = new WSClass(url);
            } catch (err) {
                return reject(new RTIexception(`Failed to connect to ${url}: ${err}`));
            }

            this.ws.onopen = async () => {
                this.connected = true;
                try {
                    await this.sendRequest('connect', {});
                    this.emit('connected');
                    resolve();
                } catch (e) {
                    reject(e);
                }
            };

            this.ws.onerror = (err: any) => {
                if (!this.connected) {
                    reject(new RTIexception(`WebSocket connection error to ${url}: ${err.message || err}`));
                }
                this.emit('error', err);
            };

            this.ws.onclose = () => {
                this.connected = false;
                this.rejectAllPending('WebSocket connection closed');
                this.emit('disconnected');
            };

            this.ws.onmessage = (evt: any) => {
                this.handleMessage(evt.data);
            };
        });
    }

    public async disconnect(): Promise<void> {
        if (!this.connected || !this.ws) return;
        this.ws.close();
        this.connected = false;
        this.federateHandle = 0;
        this.rejectAllPending('Client disconnected');
    }

    public async createFederationExecution(federationName: string, fomModules?: string | string[]): Promise<void> {
        let foms: string[] = [];
        if (typeof fomModules === 'string') foms = [fomModules];
        else if (Array.isArray(fomModules)) foms = fomModules;

        await this.sendRequest('createFederationExecution', {
            federationName,
            fomModules: foms
        });
    }

    public async destroyFederationExecution(federationName: string): Promise<void> {
        await this.sendRequest('destroyFederationExecution', { federationName });
    }

    public async joinFederationExecution(federateName: string, federationName: string, federateType = 'WebFederate'): Promise<number> {
        const res = await this.sendRequest('joinFederationExecution', {
            federateName,
            federationName,
            federateType
        });
        this.federateHandle = res.federateHandle;
        return this.federateHandle;
    }

    public async resignFederationExecution(resignAction = ResignAction.DELETE_OBJECTS_THEN_DIVEST): Promise<void> {
        await this.sendRequest('resignFederationExecution', { resignAction });
        this.federateHandle = 0;
    }

    public async getObjectClassHandle(className: string): Promise<number> {
        const res = await this.sendRequest('getObjectClassHandle', { className });
        return res.handle;
    }

    public async getAttributeHandle(classHandle: number, attributeName: string): Promise<number> {
        const res = await this.sendRequest('getAttributeHandle', { classHandle, attributeName });
        return res.handle;
    }

    public async getDimensionHandle(dimensionName: string): Promise<number> {
        const res = await this.sendRequest('getDimensionHandle', { dimensionName });
        return res.handle;
    }

    public async publishObjectClassAttributes(classHandle: number, attributeHandles: number[]): Promise<void> {
        await this.sendRequest('publishObjectClassAttributes', { classHandle, attributeHandles });
    }

    public async subscribeObjectClassAttributes(classHandle: number, attributeHandles: number[]): Promise<void> {
        await this.sendRequest('subscribeObjectClassAttributes', { classHandle, attributeHandles });
    }

    public async createRegion(dimensions: number[] = []): Promise<number> {
        const res = await this.sendRequest('createRegion', { dimensions });
        return res.regionHandle;
    }

    public async setMetricBounds(
        regionHandle: number,
        minX: number, maxX: number,
        minY: number, maxY: number,
        minZ: number, maxZ: number
    ): Promise<void> {
        await this.sendRequest('setMetricBounds', {
            regionHandle,
            minX, maxX, minY, maxY, minZ, maxZ
        });
    }

    public async subscribeObjectClassAttributesWithRegions(
        classHandle: number,
        attributeHandles: number[],
        regionHandle: number
    ): Promise<void> {
        await this.sendRequest('subscribeObjectClassAttributesWithRegions', {
            classHandle,
            attributeHandles,
            regionHandle
        });
    }

    public async registerObjectInstance(classHandle: number, objectName?: string): Promise<number> {
        const res = await this.sendRequest('registerObjectInstance', { classHandle, objectName });
        return res.objectHandle;
    }

    public async deleteObjectInstance(objectHandle: number, tag = ''): Promise<void> {
        await this.sendRequest('deleteObjectInstance', { objectHandle, tag });
    }

    public async updateAttributeValues(
        objectHandle: number,
        attributes: Record<number | string, string>,
        tag = ''
    ): Promise<void> {
        await this.sendRequest('updateAttributeValues', { objectHandle, attributes, tag });
    }

    public async updateAttributeValuesSpatial(
        objectHandle: number,
        attributes: Record<number | string, string>,
        x: number, y: number, z: number,
        tag = ''
    ): Promise<void> {
        await this.sendRequest('updateAttributeValuesSpatial', {
            objectHandle,
            attributes,
            x, y, z,
            tag
        });
    }

    public async isAttributeOwnedByFederate(objectHandle: number, attributeHandle: number): Promise<boolean> {
        const res = await this.sendRequest('isAttributeOwnedByFederate', { objectHandle, attributeHandle });
        return Boolean(res.isOwned);
    }

    public async unconditionalAttributeOwnershipDivestiture(objectHandle: number, attributeHandles: number[]): Promise<void> {
        await this.sendRequest('unconditionalAttributeOwnershipDivestiture', { objectHandle, attributeHandles });
    }

    public async attributeOwnershipAcquisition(objectHandle: number, attributeHandles: number[], tag = ''): Promise<void> {
        await this.sendRequest('attributeOwnershipAcquisition', { objectHandle, attributeHandles, tag });
    }

    public async timeAdvanceRequest(time: number): Promise<void> {
        await this.sendRequest('timeAdvanceRequest', { time });
    }

    public async evokeCallback(timeoutSeconds = 0.05): Promise<boolean> {
        return true;
    }

    public getFederateHandle(): number {
        return this.federateHandle;
    }

    private rejectAllPending(reason: string): void {
        for (const [id, entry] of this.pendingRequests.entries()) {
            if ((entry as any).timer) {
                clearTimeout((entry as any).timer);
            }
            this.pendingRequests.delete(id);
            entry.reject(new RTIexception(reason, 'NotConnected'));
        }
    }

    private sendRequest(action: string, params: any, timeoutMs = 10000): Promise<any> {
        if (!this.connected || !this.ws) {
            return Promise.reject(new RTIexception('Not connected to Valhalla CRC', 'NotConnected'));
        }

        const id = ++this.reqId;
        const payload = JSON.stringify({ id, action, params });

        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                if (this.pendingRequests.delete(id)) {
                    reject(new RTIexception(`Request '${action}' timed out after ${timeoutMs}ms`, 'RequestTimeout'));
                }
            }, timeoutMs);
            (this.pendingRequests as any).set(id, { resolve, reject, timer });
            this.ws.send(payload);
        });
    }

    private handleMessage(raw: any): void {
        try {
            const data = typeof raw === 'string' ? JSON.parse(raw) : JSON.parse(raw.toString());

            // Handle Response to pending request
            if (data.id && this.pendingRequests.has(data.id)) {
                const entry: any = this.pendingRequests.get(data.id)!;
                if (entry.timer) {
                    clearTimeout(entry.timer);
                }
                const { resolve, reject } = entry;
                this.pendingRequests.delete(data.id);

                if (data.status === 'OK') {
                    resolve(data.result || {});
                } else {
                    reject(new RTIexception(data.message || data.error || 'Operation failed', data.code));
                }
                return;
            }

            // Handle Asynchronous Callback from CRC
            if (data.callback) {
                this.handleCallback(data.callback, data.params || {});
            }
        } catch (e) {
            console.error('[RTIambassador] Error parsing incoming WebSocket frame:', e);
        }
    }

    private handleCallback(callback: string, params: any): void {
        this.emit(callback, params);

        if (!this.fedAmb) return;

        switch (callback) {
            case 'discoverObjectInstance':
                if (this.fedAmb.discoverObjectInstance) {
                    this.fedAmb.discoverObjectInstance(params.object, params.class, params.name);
                }
                break;
            case 'reflectAttributeValues':
                if (this.fedAmb.reflectAttributeValues) {
                    this.fedAmb.reflectAttributeValues(params.object, params.attributes, params.tag);
                }
                break;
            case 'removeObjectInstance':
                if (this.fedAmb.removeObjectInstance) {
                    this.fedAmb.removeObjectInstance(params.object, params.tag);
                }
                break;
            case 'timeAdvanceGrant':
                if (this.fedAmb.timeAdvanceGrant) {
                    this.fedAmb.timeAdvanceGrant(params.time);
                }
                break;
            case 'announceSynchronizationPoint':
                if (this.fedAmb.announceSynchronizationPoint) {
                    this.fedAmb.announceSynchronizationPoint(params.label, params.tag);
                }
                break;
            case 'federationSynchronized':
                if (this.fedAmb.federationSynchronized) {
                    this.fedAmb.federationSynchronized(params.label);
                }
                break;
        }
    }
}
