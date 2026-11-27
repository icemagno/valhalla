package crc

import (
	"encoding/binary"
	"errors"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

// handleObjectManagement handles IEEE 1516-2025 Clause 6 services
func (ctx *nativeContext) handleObjectManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgRegisterObject:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var regReq common.RegisterObjectInstancePayload
		if err := regReq.UnmarshalBinary(payload); err != nil || regReq.ClassHandle == 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid register object request")); err != nil {
				return true, err
			}
			return true, nil
		}
		if _, exists := ctx.currentFed.OMT().GetObjectClass(regReq.ClassHandle); !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, common.ErrObjectClassNotDefined); err != nil {
				return true, err
			}
			return true, nil
		}
		obj, err := ctx.currentFed.RegisterAndBroadcastObject(int32(regReq.ClassHandle), regReq.ObjectName, ctx.federateID)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 409, err); err != nil {
				return true, err
			}
			return true, nil
		}

		var respPayload [4]byte
		binary.LittleEndian.PutUint32(respPayload[0:4], uint32(obj.Handle))

		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			PayloadLen: 4,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload[:], ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Object registered: handle=%d, class=%d, name=%s", obj.Handle, regReq.ClassHandle, obj.Name)
		return true, nil

	case NativeMsgDeleteObjectInstance:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var delReq common.DeleteObjectInstancePayload
		if err := delReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid delete object instance payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		obj, err := ctx.currentFed.DeleteAndBroadcastObject(int32(delReq.ObjectHandle), ctx.federateID, delReq.Tag)
		if err != nil {
			if errors.Is(err, common.ErrObjectInstanceNotKnown) {
				if err := sendNativeError(ctx.session, hdr.ReqID, 404, err); err != nil {
					return true, err
				}
			} else if errors.Is(err, common.ErrDeletePrivilegeNotHeld) {
				if err := sendNativeError(ctx.session, hdr.ReqID, 403, err); err != nil {
					return true, err
				}
			} else {
				if err := sendNativeError(ctx.session, hdr.ReqID, 500, err); err != nil {
					return true, err
				}
			}
			return true, nil
		}

		var globalRoot [32]byte
		respFlags := common.FlagNativeSSZ
		if ctx.currentFed.IsMerkleAuditEnabled() {
			globalRoot = ctx.currentFed.StateTree().GetGlobalRoot()
			respFlags |= common.FlagNativeMerkle
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      respFlags,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			MerkleRoot: globalRoot,
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Object [%d] '%s' deleted by federate [%d]", obj.Handle, obj.Name, ctx.federateID)
		return true, nil

	case NativeMsgUpdateAttributes:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var uavReq common.UpdateAttributesPayload
		if err := uavReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid update attributes payload")); err != nil {
				return true, err
			}
			return true, nil
		}

		globalRoot, err := ctx.currentFed.UpdateAndRouteAttributes(
			ctx.federateID,
			uavReq.ObjectHandle,
			uavReq.Attributes,
			uavReq.Tag,
			uavReq.HasTime,
			uavReq.Time,
			nil,
			hdr.MerkleRoot,
		)
		if err != nil {
			if errors.Is(err, common.ErrObjectInstanceNotKnown) {
				if sendErr := sendNativeError(ctx.session, hdr.ReqID, 404, err); sendErr != nil {
					return true, sendErr
				}
			} else if errors.Is(err, core.ErrNotAttributeOwner) {
				if sendErr := sendNativeError(ctx.session, hdr.ReqID, 403, err); sendErr != nil {
					return true, sendErr
				}
			} else {
				logCRCNative.Warn("failed to update attributes for object %d: %v", uavReq.ObjectHandle, err)
				if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
					return true, sendErr
				}
			}
			return true, nil
		}

		respFlags := common.FlagNativeSSZ
		if ctx.currentFed.IsMerkleAuditEnabled() {
			respFlags |= common.FlagNativeMerkle
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      respFlags,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			MerkleRoot: globalRoot,
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case NativeMsgSendInteraction:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var interReq common.SendInteractionPayload
		if err := interReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid send interaction payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if _, exists := ctx.currentFed.OMT().GetInteractionClass(interReq.ClassHandle); !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, common.ErrInteractionClassNotDefined); err != nil {
				return true, err
			}
			return true, nil
		}

		if err := ctx.currentFed.RouteInteraction(ctx.federateID, interReq.ClassHandle, interReq.Parameters, interReq.Tag, interReq.HasTime, interReq.Time); err != nil {
			logCRCNative.Warn("failed to route interaction %d: %v", interReq.ClassHandle, err)
			code := uint16(400)
			if errors.Is(err, core.ErrNotPublished) {
				code = 403
			}
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, code, err); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}

		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

// handleOwnershipManagement handles IEEE 1516-2025 Clause 7 services
func (ctx *nativeContext) handleOwnershipManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgDivestOwnership:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.DivestOwnershipPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid divest ownership payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		attrs := convertToAttrHandles(req.Attributes)
		if err := ctx.currentFed.OwnershipManager().UnconditionalDivestiture(common.ObjectInstanceHandle(req.ObjectHandle), attrs, common.FederateHandle(ctx.federateID)); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case NativeMsgAcquireOwnership:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.AcquireOwnershipPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid acquire ownership payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		attrs := convertToAttrHandles(req.Attributes)
		if err := ctx.currentFed.OwnershipManager().AcquireAttributeOwnership(common.ObjectInstanceHandle(req.ObjectHandle), attrs, common.FederateHandle(ctx.federateID)); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case NativeMsgQueryAttributeOwnership:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.QueryOwnershipPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid query ownership payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		owner, isOwned := ctx.currentFed.OwnershipManager().QueryAttributeOwnership(common.ObjectInstanceHandle(req.ObjectHandle), common.AttributeHandle(req.AttributeHandle))
		respPayload, err := common.QueryOwnershipResponsePayload{
			ObjectHandle:    req.ObjectHandle,
			AttributeHandle: req.AttributeHandle,
			OwnerHandle:     uint32(owner),
			IsOwned:         isOwned,
		}.MarshalBinary()
		if err != nil {
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, 500, err); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}

		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			PayloadLen: uint32(len(respPayload)),
			MsgType:    NativeMsgAttributeOwnershipResponse,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

