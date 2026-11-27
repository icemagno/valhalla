package crc

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

// Message types for the native Valhalla HLA-4 protocol
const (
	NativeMsgCreateFederation       = common.NativeMsgCreateFederation
	NativeMsgDestroyFederation      = common.NativeMsgDestroyFederation
	NativeMsgJoin                   = common.NativeMsgJoin
	NativeMsgJoinSuccess            = common.NativeMsgJoinSuccess
	NativeMsgRegisterSyncPoint      = common.NativeMsgRegisterSyncPoint
	NativeMsgAnnounceSyncPoint      = common.NativeMsgAnnounceSyncPoint
	NativeMsgAchieveSyncPoint       = common.NativeMsgAchieveSyncPoint
	NativeMsgSyncPointAchieved      = common.NativeMsgSyncPointAchieved
	NativeMsgFederationSynchronized = common.NativeMsgFederationSynchronized
	NativeMsgPublishObjectClass          = common.NativeMsgPublishObjectClass
	NativeMsgUnpublishObjectClass        = common.NativeMsgUnpublishObjectClass
	NativeMsgPublishInteractionClass     = common.NativeMsgPublishInteractionClass
	NativeMsgUnpublishInteractionClass   = common.NativeMsgUnpublishInteractionClass
	NativeMsgSubscribeObjectClassAttrs   = common.NativeMsgSubscribeObjectClassAttrs
	NativeMsgUnsubscribeObjectClass      = common.NativeMsgUnsubscribeObjectClass
	NativeMsgSubscribeInteractionClass   = common.NativeMsgSubscribeInteractionClass
	NativeMsgUnsubscribeInteractionClass = common.NativeMsgUnsubscribeInteractionClass
	NativeMsgRegisterObject         = common.NativeMsgRegisterObject
	NativeMsgDiscoverObject         = common.NativeMsgDiscoverObject
	NativeMsgDeleteObjectInstance   = common.NativeMsgDeleteObjectInstance
	NativeMsgRemoveObjectInstance   = common.NativeMsgRemoveObjectInstance
	NativeMsgUpdateAttributes       = common.NativeMsgUpdateAttributes
	NativeMsgReflectAttributes      = common.NativeMsgReflectAttributes
	NativeMsgSendInteraction        = common.NativeMsgSendInteraction
	NativeMsgReceiveInteraction     = common.NativeMsgReceiveInteraction
	NativeMsgEnableTimeRegulation    = common.NativeMsgEnableTimeRegulation
	NativeMsgTimeRegulationEnabled   = common.NativeMsgTimeRegulationEnabled
	NativeMsgDisableTimeRegulation   = common.NativeMsgDisableTimeRegulation
	NativeMsgEnableTimeConstrained   = common.NativeMsgEnableTimeConstrained
	NativeMsgTimeConstrainedEnabled  = common.NativeMsgTimeConstrainedEnabled
	NativeMsgDisableTimeConstrained  = common.NativeMsgDisableTimeConstrained
	NativeMsgTimeAdvanceRequest      = common.NativeMsgTimeAdvanceRequest
	NativeMsgTimeAdvanceGrant        = common.NativeMsgTimeAdvanceGrant
	NativeMsgNextMessageRequest      = common.NativeMsgNextMessageRequest
	NativeMsgFlushQueueRequest       = common.NativeMsgFlushQueueRequest
	NativeMsgHeartbeat               = common.NativeMsgHeartbeat
	NativeMsgDivestOwnership         = common.NativeMsgDivestOwnership
	NativeMsgAcquireOwnership        = common.NativeMsgAcquireOwnership
	NativeMsgQueryAttributeOwnership = common.NativeMsgQueryAttributeOwnership
	NativeMsgAttributeOwnershipResponse = common.NativeMsgAttributeOwnershipResponse
	NativeMsgCreateRegion            = common.NativeMsgCreateRegion
	NativeMsgRegionCreated           = common.NativeMsgRegionCreated
	NativeMsgSetRangeBounds          = common.NativeMsgSetRangeBounds
	NativeMsgSetMetricBounds         = common.NativeMsgSetMetricBounds
	NativeMsgSubscribeWithRegion     = common.NativeMsgSubscribeWithRegion
	NativeMsgUpdateAttributesSpatial = common.NativeMsgUpdateAttributesSpatial
	NativeMsgResign                  = common.NativeMsgResign
	NativeMsgSuccessOK               = common.NativeMsgSuccessOK
	NativeMsgError                   = common.NativeMsgError
)

func sendNativeError(session *NativeSession, reqID uint16, code uint16, err error) error {
	errPayload, marshalErr := common.ErrorResponsePayload{
		ErrorCode: code,
		Message:   err.Error(),
	}.MarshalBinary()
	if marshalErr != nil {
		logCRCNative.Error("failed to marshal ErrorResponsePayload: %v", marshalErr)
		return marshalErr
	}
	respHdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      0x01,
		PayloadLen: uint32(len(errPayload)),
		MsgType:    NativeMsgError,
		ReqID:      reqID,
	}
	if sendErr := session.SendMessage(respHdr, errPayload); sendErr != nil {
		logCRCNative.Warn("failed to send error response (reqID=%d, code=%d): %v", reqID, code, sendErr)
		return sendErr
	}
	return nil
}

func sendNativeResponse(session *NativeSession, hdr NativeHeader, payload []byte, federateID int32) error {
	if err := session.SendMessage(hdr, payload); err != nil {
		logCRCNative.Warn("failed to send response (msgType=%d, reqID=%d) to federate %d: %v", hdr.MsgType, hdr.ReqID, federateID, err)
		return err
	}
	return nil
}

func dispatchTimeGrants(currentFed *FederationExecution, grants []core.TimeGrantEvent) {
	for _, grant := range grants {
		fed, ok := currentFed.GetFederate(int32(grant.Federate))
		if !ok || fed.Session == nil {
			continue
		}
		// Deliver any queued TSO messages first in strictly non-decreasing time order
		for _, msg := range grant.Messages {
			if err := fed.Session.SendNotificationWithMerkle(msg.MsgType, msg.Payload, msg.MerkleRoot); err != nil {
				logCRCNative.Warn("failed to deliver TSO message %d to federate %d: %v", msg.MsgType, grant.Federate, err)
			}
		}
		// Deliver TimeAdvanceGrant notification
		grantPayload, err := common.TimeFloat64Payload{Time: grant.Time}.MarshalBinary()
		if err != nil {
			logCRCNative.Error("failed to marshal TimeFloat64Payload: %v", err)
			continue
		}
		if err := fed.Session.SendNotification(NativeMsgTimeAdvanceGrant, grantPayload); err != nil {
			logCRCNative.Warn("failed to deliver TimeAdvanceGrant to federate %d: %v", grant.Federate, err)
		}
	}
}

func convertToAttrHandles(attrs []uint32) []common.AttributeHandle {
	res := make([]common.AttributeHandle, len(attrs))
	for i, a := range attrs {
		res[i] = common.AttributeHandle(a)
	}
	return res
}

// HandleNativeConnection manages a connected native client using the Valhalla binary protocol.
// Network framing and protocol dispatch are cleanly separated into IEEE 1516-2025 service dispatchers.
func HandleNativeConnection(conn net.Conn, reader io.Reader, fedManager *FederationManager) error {
	session := NewNativeSession(conn)
	defer session.Close()

	ctx := &nativeContext{
		session:    session,
		fedManager: fedManager,
		federateID: -1,
	}

	for {
		hdr, err := ReadNativeHeader(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				if ctx.currentFed != nil && ctx.federateID > 0 {
					if _, err := ctx.currentFed.Resign(ctx.federateID); err != nil {
						logCRCNative.Warn("failed to resign federate %d on disconnect: %v", ctx.federateID, err)
					}
				}
				return nil
			}
			return fmt.Errorf("failed to read native header: %w", err)
		}

		var payload []byte
		if hdr.PayloadLen > common.MaxNativePayloadLen {
			return fmt.Errorf("native payload length %d exceeds maximum %d", hdr.PayloadLen, common.MaxNativePayloadLen)
		}
		if hdr.PayloadLen > 0 {
			payload = make([]byte, hdr.PayloadLen)
			if _, err := io.ReadFull(reader, payload); err != nil {
				return fmt.Errorf("failed to read native payload (%d bytes): %w", hdr.PayloadLen, err)
			}
		}

		if (hdr.Flags & common.FlagNativePQC) != 0 {
			if err := sendNativeError(session, hdr.ReqID, 400, errors.New("post-quantum cryptography (PQC) framing is not supported in this version; see Roadmap")); err != nil {
				return err
			}
			continue
		}

		if ctx.currentFed != nil && ctx.federateID > 0 {
			ctx.currentFed.TimeManager().Heartbeat(common.FederateHandle(ctx.federateID))
		}

		if handled, err := ctx.handleFederationManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleDeclarationManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleObjectManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleOwnershipManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleTimeManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleDDMManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleMetadataManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}

		if err := sendNativeError(session, hdr.ReqID, 400, fmt.Errorf("unknown message type: 0x%02x", hdr.MsgType)); err != nil {
			return err
		}
	}
}
