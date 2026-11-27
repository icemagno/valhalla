package crc

import (
	"errors"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

type porticoContext struct {
	session    *PorticoSession
	fedManager *FederationManager
	currentFed *FederationExecution
	federateID int32
}

// handleFederationManagement handles Portico federation lifecycle (Create, Join, Resign, Destroy, Disconnect)
func (ctx *porticoContext) handleFederationManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgRtiProbe, wire.MsgConnect:
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Portico %s acknowledged (reqID=%d)", hdr.MessageType.String(), hdr.RequestId)
		return true, nil

	case wire.MsgCreateFederation:
		var fedName string
		obj, err := wire.DeserializePorticoControlMessage(payload)
		if err != nil {
			logCRCPortico.Error("Error deserializing CreateFederation: %v", err)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "CouldNotOpenFDD: malformed CreateFederation request"); err != nil {
				return true, err
			}
			return true, nil
		}
		if n, ok := obj.Get("federationName").(string); ok && n != "" {
			fedName = n
		}
		if fedName == "" {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "CouldNotOpenFDD: empty federation name"); err != nil {
				return true, err
			}
			return true, nil
		}

		fedExec, err := ctx.fedManager.CreateFederation(fedName)
		if err != nil {
			logCRCPortico.Error("CreateFederation failed for [%s]: %v", fedName, err)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
				return true, err
			}
			return true, nil
		}
		ctx.currentFed = fedExec

		// Ingest any FOM modules provided in the CreateFederation request
		if obj != nil {
			for _, key := range []string{"rawFomModules", "modules", "fomModuleLocations", "rawFomModuleLocations"} {
				if val := obj.Get(key); val != nil {
					ingestPorticoFOMModules(ctx.currentFed.OMT(), val)
				}
			}
		}

		respPayload := wire.BuildCreateFederationResponsePayload(ctx.currentFed.Handle, fedName)
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federation [%s] created with handle [%d]", fedName, ctx.currentFed.Handle)
		return true, nil

	case wire.MsgJoinFederation:
		var fedName, nodeName, nodeType string
		nodeType = "ieee1516e"

		obj, err := wire.DeserializePorticoControlMessage(payload)
		if err != nil {
			logCRCPortico.Error("Error deserializing JoinFederation request: %v", err)
		}
		if obj != nil {
			if n, ok := obj.Get("federationName").(string); ok && n != "" {
				fedName = n
			}
			if n, ok := obj.Get("federateName").(string); ok && n != "" {
				nodeName = n
			}
			if n, ok := obj.Get("federateType").(string); ok && n != "" {
				nodeType = n
			}
		}

		if nodeName == "" {
			nodeName = wire.ExtractFederateNameFromJoinPayload(payload)
		}

		if fedName == "" || nodeName == "" {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: federation and federate names are mandatory"); err != nil {
				return true, err
			}
			return true, nil
		}

		fedExec, exists := ctx.fedManager.GetFederation(fedName)
		if !exists {
			logCRCPortico.Error("Join error: federation [%s] does not exist", fedName)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederationExecutionDoesNotExist: "+fedName); err != nil {
				return true, err
			}
			return true, nil
		}

		// Ingest join-time FOM modules
		if obj != nil {
			for _, key := range []string{"rawFomModules", "rawJoinObjectModels", "modules", "fomModuleLocations", "rawFomModuleLocations"} {
				if val := obj.Get(key); val != nil {
					ingestPorticoFOMModules(fedExec.OMT(), val)
				}
			}
		}

		if ctx.currentFed != nil && ctx.federateID > 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateAlreadyExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}

		fed, err := fedExec.Join(nodeName, nodeType, ProtocolPortico, ctx.session)
		if err != nil {
			logCRCPortico.Error("Join error for %s: %v", nodeName, err)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
				return true, err
			}
			return true, nil
		}

		ctx.currentFed = fedExec
		ctx.federateID = fed.ID
		ctx.session.SetFederateID(ctx.federateID)
		ctx.session.SetFederationHandle(ctx.currentFed.Handle)

		wpPayload, err := wire.BuildWelcomePackPayloadWithFOM(ctx.currentFed.Handle, ctx.federateID, ctx.currentFed.OMT(), fedName, nodeName, nodeType)
		if err != nil {
			logCRCPortico.Error("Error generating WelcomePack: %v", err)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: failed to build WelcomePack"); err != nil {
				return true, err
			}
			return true, nil
		}

		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wpPayload); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%s] joined with handle [%d] to federation [%s]", nodeName, ctx.federateID, fedName)
		return true, nil

	case wire.MsgResignFederation:
		if ctx.currentFed != nil && ctx.federateID > 0 {
			if _, err := ctx.currentFed.Resign(ctx.federateID); err != nil {
				logCRCPortico.Warn("failed to resign federate %d: %v", ctx.federateID, err)
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] resigned gracefully", ctx.federateID)
		ctx.federateID = 0
		return true, nil

	case wire.MsgDestroyFederation:
		var fedName string
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if fn, ok := obj.Get("federationName").(string); ok {
				fedName = fn
			}
		}
		if fedName == "" && ctx.currentFed != nil {
			fedName = ctx.currentFed.Name
		}
		if fedName == "" {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederationExecutionDoesNotExist"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		if err := ctx.fedManager.DestroyFederation(fedName); err != nil {
			logCRCPortico.Warn("failed to destroy federation [%s]: %v", fedName, err)
			errMsg := "RTIinternalError: " + err.Error()
			if errors.Is(err, ErrFederatesCurrentlyJoined) || errors.Is(err, common.ErrFederatesCurrentlyJoined) {
				errMsg = "FederatesCurrentlyJoined: " + fedName
			} else if errors.Is(err, ErrFederationExecutionDoesNotExist) || errors.Is(err, common.ErrFederationExecutionDoesNotExist) {
				errMsg = "FederationExecutionDoesNotExist: " + fedName
			}
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, errMsg); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		logCRCPortico.Info("Federation [%s] destroyed", fedName)
		ctx.currentFed = nil
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgDisconnect:
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Connection disconnected gracefully")
		return true, nil
	}
	return false, nil
}
