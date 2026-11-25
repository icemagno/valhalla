(function (root, factory) {
    if (typeof define === 'function' && define.amd) {
        define([], factory);
    } else if (typeof module === 'object' && module.exports) {
        module.exports = factory();
    } else {
        root.ValhallaHLA = factory();
    }
}(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    var CallbackModel = {
        HLA_IMMEDIATE: 'HLA_IMMEDIATE',
        HLA_EVOKED: 'HLA_EVOKED'
    };

    var OrderType = {
        RECEIVE: 'RECEIVE',
        TIMESTAMP: 'TIMESTAMP'
    };

    var ResignAction = {
        UNCONDITIONALLY_DIVEST_ATTRIBUTES: 'UNCONDITIONALLY_DIVEST_ATTRIBUTES',
        DELETE_OBJECTS: 'DELETE_OBJECTS',
        CANCEL_PENDING_OWNERSHIP_ACQUISITIONS: 'CANCEL_PENDING_OWNERSHIP_ACQUISITIONS',
        DELETE_OBJECTS_THEN_DIVEST: 'DELETE_OBJECTS_THEN_DIVEST',
        CANCEL_THEN_DELETE_THEN_DIVEST: 'CANCEL_THEN_DELETE_THEN_DIVEST',
        NO_ACTION: 'NO_ACTION'
    };

    function RTIexception(message, code) {
        var err = new Error(message);
        err.name = code || 'RTIexception';
        err.code = code;
        Object.setPrototypeOf(err, RTIexception.prototype);
        return err;
    }
    RTIexception.prototype = Object.create(Error.prototype);
    RTIexception.prototype.constructor = RTIexception;

    function NullFederateAmbassador() {}
    NullFederateAmbassador.prototype.discoverObjectInstance = function (object, objectClass, objectName) {};
    NullFederateAmbassador.prototype.reflectAttributeValues = function (object, attributes, tag) {};
    NullFederateAmbassador.prototype.removeObjectInstance = function (object, tag) {};
    NullFederateAmbassador.prototype.timeAdvanceGrant = function (time) {};
    NullFederateAmbassador.prototype.announceSynchronizationPoint = function (label, tag) {};
    NullFederateAmbassador.prototype.federationSynchronized = function (label) {};

    function RTIambassador(defaultUrl) {
        this.defaultUrl = defaultUrl || 'ws://127.0.0.1:52296/ws';
        this.ws = null;
        this.reqId = 0;
        this.pendingRequests = new Map();
        this.listeners = new Map();
        this.fedAmb = null;
        this.federateHandle = 0;
        this.connected = false;
    }

    RTIambassador.prototype.on = function (event, handler) {
        if (!this.listeners.has(event)) {
            this.listeners.set(event, []);
        }
        this.listeners.get(event).push(handler);
        return this;
    };

    RTIambassador.prototype.off = function (event, handler) {
        var list = this.listeners.get(event);
        if (list) {
            this.listeners.set(event, list.filter(function (h) { return h !== handler; }));
        }
        return this;
    };

    RTIambassador.prototype.emit = function (event) {
        var args = Array.prototype.slice.call(arguments, 1);
        var list = this.listeners.get(event);
        if (list) {
            for (var i = 0; i < list.length; i++) {
                try {
                    list[i].apply(null, args);
                } catch (e) {
                    console.error('[RTIambassador] Error in listener for ' + event + ':', e);
                }
            }
        }
    };

    RTIambassador.prototype.connect = function (fedAmbOrUrl, maybeUrl) {
        var self = this;
        if (self.connected) return Promise.resolve();

        var url = self.defaultUrl;
        if (typeof fedAmbOrUrl === 'string') {
            url = fedAmbOrUrl;
        } else if (fedAmbOrUrl) {
            self.fedAmb = fedAmbOrUrl;
            if (maybeUrl) url = maybeUrl;
        }

        var WSClass;
        if (typeof WebSocket !== 'undefined') {
            WSClass = WebSocket;
        } else if (typeof globalThis !== 'undefined' && globalThis.WebSocket) {
            WSClass = globalThis.WebSocket;
        } else {
            try {
                WSClass = require('ws');
            } catch (e) {
                return Promise.reject(new RTIexception("WebSocket is not available. Install 'ws' for Node.js."));
            }
        }

        return new Promise(function (resolve, reject) {
            try {
                self.ws = new WSClass(url);
            } catch (err) {
                return reject(new RTIexception('Failed to connect to ' + url + ': ' + err));
            }

            self.ws.onopen = function () {
                self.connected = true;
                self.sendRequest('connect', {})
                    .then(function () {
                        self.emit('connected');
                        resolve();
                    })
                    .catch(reject);
            };

            self.ws.onerror = function (err) {
                if (!self.connected) {
                    reject(new RTIexception('WebSocket connection error to ' + url + ': ' + (err.message || err)));
                }
                self.emit('error', err);
            };

            self.ws.onclose = function () {
                self.connected = false;
                self.emit('disconnected');
            };

            self.ws.onmessage = function (evt) {
                self.handleMessage(evt.data);
            };
        });
    };

    RTIambassador.prototype.disconnect = function () {
        if (!this.connected || !this.ws) return Promise.resolve();
        this.ws.close();
        this.connected = false;
        this.federateHandle = 0;
        return Promise.resolve();
    };

    RTIambassador.prototype.createFederationExecution = function (federationName, fomModules) {
        var foms = [];
        if (typeof fomModules === 'string') foms = [fomModules];
        else if (Array.isArray(fomModules)) foms = fomModules;

        return this.sendRequest('createFederationExecution', {
            federationName: federationName,
            fomModules: foms
        });
    };

    RTIambassador.prototype.destroyFederationExecution = function (federationName) {
        return this.sendRequest('destroyFederationExecution', { federationName: federationName });
    };

    RTIambassador.prototype.joinFederationExecution = function (federateName, federationName, federateType) {
        var self = this;
        return this.sendRequest('joinFederationExecution', {
            federateName: federateName,
            federationName: federationName,
            federateType: federateType || 'WebFederate'
        }).then(function (res) {
            self.federateHandle = res.federateHandle;
            return res.federateHandle;
        });
    };

    RTIambassador.prototype.resignFederationExecution = function (resignAction) {
        var self = this;
        return this.sendRequest('resignFederationExecution', {
            resignAction: resignAction || ResignAction.DELETE_OBJECTS_THEN_DIVEST
        }).then(function () {
            self.federateHandle = 0;
        });
    };

    RTIambassador.prototype.getObjectClassHandle = function (className) {
        return this.sendRequest('getObjectClassHandle', { className: className })
            .then(function (res) { return res.handle; });
    };

    RTIambassador.prototype.getAttributeHandle = function (classHandle, attributeName) {
        return this.sendRequest('getAttributeHandle', { classHandle: classHandle, attributeName: attributeName })
            .then(function (res) { return res.handle; });
    };

    RTIambassador.prototype.getDimensionHandle = function (dimensionName) {
        return this.sendRequest('getDimensionHandle', { dimensionName: dimensionName })
            .then(function (res) { return res.handle; });
    };

    RTIambassador.prototype.publishObjectClassAttributes = function (classHandle, attributeHandles) {
        return this.sendRequest('publishObjectClassAttributes', {
            classHandle: classHandle,
            attributeHandles: attributeHandles
        });
    };

    RTIambassador.prototype.subscribeObjectClassAttributes = function (classHandle, attributeHandles) {
        return this.sendRequest('subscribeObjectClassAttributes', {
            classHandle: classHandle,
            attributeHandles: attributeHandles
        });
    };

    RTIambassador.prototype.createRegion = function (dimensions) {
        return this.sendRequest('createRegion', { dimensions: dimensions || [] })
            .then(function (res) { return res.regionHandle; });
    };

    RTIambassador.prototype.setMetricBounds = function (regionHandle, minX, maxX, minY, maxY, minZ, maxZ) {
        return this.sendRequest('setMetricBounds', {
            regionHandle: regionHandle,
            minX: minX, maxX: maxX,
            minY: minY, maxY: maxY,
            minZ: minZ, maxZ: maxZ
        });
    };

    RTIambassador.prototype.subscribeObjectClassAttributesWithRegions = function (classHandle, attributeHandles, regionHandle) {
        return this.sendRequest('subscribeObjectClassAttributesWithRegions', {
            classHandle: classHandle,
            attributeHandles: attributeHandles,
            regionHandle: regionHandle
        });
    };

    RTIambassador.prototype.registerObjectInstance = function (classHandle, objectName) {
        return this.sendRequest('registerObjectInstance', {
            classHandle: classHandle,
            objectName: objectName
        }).then(function (res) { return res.objectHandle; });
    };

    RTIambassador.prototype.deleteObjectInstance = function (objectHandle, tag) {
        return this.sendRequest('deleteObjectInstance', {
            objectHandle: objectHandle,
            tag: tag || ''
        });
    };

    RTIambassador.prototype.updateAttributeValues = function (objectHandle, attributes, tag) {
        return this.sendRequest('updateAttributeValues', {
            objectHandle: objectHandle,
            attributes: attributes,
            tag: tag || ''
        });
    };

    RTIambassador.prototype.updateAttributeValuesSpatial = function (objectHandle, attributes, x, y, z, tag) {
        return this.sendRequest('updateAttributeValuesSpatial', {
            objectHandle: objectHandle,
            attributes: attributes,
            x: x, y: y, z: z,
            tag: tag || ''
        });
    };

    RTIambassador.prototype.isAttributeOwnedByFederate = function (objectHandle, attributeHandle) {
        return this.sendRequest('isAttributeOwnedByFederate', {
            objectHandle: objectHandle,
            attributeHandle: attributeHandle
        }).then(function (res) { return Boolean(res.isOwned); });
    };

    RTIambassador.prototype.unconditionalAttributeOwnershipDivestiture = function (objectHandle, attributeHandles) {
        return this.sendRequest('unconditionalAttributeOwnershipDivestiture', {
            objectHandle: objectHandle,
            attributeHandles: attributeHandles
        });
    };

    RTIambassador.prototype.attributeOwnershipAcquisition = function (objectHandle, attributeHandles, tag) {
        return this.sendRequest('attributeOwnershipAcquisition', {
            objectHandle: objectHandle,
            attributeHandles: attributeHandles,
            tag: tag || ''
        });
    };

    RTIambassador.prototype.timeAdvanceRequest = function (time) {
        return this.sendRequest('timeAdvanceRequest', { time: time });
    };

    RTIambassador.prototype.evokeCallback = function () {
        return Promise.resolve(true);
    };

    RTIambassador.prototype.getFederateHandle = function () {
        return this.federateHandle;
    };

    RTIambassador.prototype.sendRequest = function (action, params) {
        var self = this;
        if (!self.connected || !self.ws) {
            return Promise.reject(new RTIexception('Not connected to Valhalla CRC', 'NotConnected'));
        }

        var id = ++self.reqId;
        var payload = JSON.stringify({ id: id, action: action, params: params });

        return new Promise(function (resolve, reject) {
            self.pendingRequests.set(id, { resolve: resolve, reject: reject });
            self.ws.send(payload);
        });
    };

    RTIambassador.prototype.handleMessage = function (raw) {
        try {
            var data = typeof raw === 'string' ? JSON.parse(raw) : JSON.parse(raw.toString());

            if (data.id && this.pendingRequests.has(data.id)) {
                var p = this.pendingRequests.get(data.id);
                this.pendingRequests.delete(data.id);

                if (data.status === 'OK') {
                    p.resolve(data.result || {});
                } else {
                    p.reject(new RTIexception(data.message || data.error || 'Operation failed', data.code));
                }
                return;
            }

            if (data.callback) {
                this.handleCallback(data.callback, data.params || {});
            }
        } catch (e) {
            console.error('[RTIambassador] Error parsing incoming WebSocket frame:', e);
        }
    };

    RTIambassador.prototype.handleCallback = function (callback, params) {
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
    };

    return {
        CallbackModel: CallbackModel,
        OrderType: OrderType,
        ResignAction: ResignAction,
        RTIexception: RTIexception,
        NullFederateAmbassador: NullFederateAmbassador,
        RTIambassador: RTIambassador
    };
}));
