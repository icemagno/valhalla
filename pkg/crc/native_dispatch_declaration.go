package crc

import (
	"errors"

	"github.com/icemagno/valhalla/pkg/common"
)

// handleDeclarationManagement handles IEEE 1516-2025 Clause 5 services
func (ctx *nativeContext) handleDeclarationManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgPublishObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var pubReq common.PublishObjectClassPayload
		if err := pubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid publish object class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		err := ctx.currentFed.Router().PublishObjectClass(common.FederateHandle(ctx.federateID), pubReq.ClassHandle, pubReq.AttributeHandles)
		if err != nil {
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

	case NativeMsgUnpublishObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var unpubReq common.PublishObjectClassPayload
		if err := unpubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid unpublish object class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.Router().UnpublishObjectClass(common.FederateHandle(ctx.federateID), unpubReq.ClassHandle); err != nil {
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
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

	case NativeMsgSubscribeObjectClassAttrs:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var subReq common.SubscribeObjectClassPayload
		if err := subReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid subscribe object class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		err := ctx.currentFed.Router().SubscribeObjectClass(common.FederateHandle(ctx.federateID), subReq.ClassHandle, subReq.AttributeHandles, subReq.Active)
		if err != nil {
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

		// Catch-up Discovery: notify the subscriber about all existing instances matching the hierarchy
		if subReq.Active {
			if err := ctx.currentFed.DispatchCatchUpDiscovery(ctx.session, subReq.ClassHandle); err != nil {
				return true, err
			}
		}
		return true, nil

	case NativeMsgUnsubscribeObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var unsubReq common.PublishObjectClassPayload
		if err := unsubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid unsubscribe object class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.Router().UnsubscribeObjectClass(common.FederateHandle(ctx.federateID), unsubReq.ClassHandle); err != nil {
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
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

	case NativeMsgPublishInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var pubReq common.InteractionClassPayload
		if err := pubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid publish interaction class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		err := ctx.currentFed.Router().PublishInteractionClass(common.FederateHandle(ctx.federateID), pubReq.ClassHandle)
		if err != nil {
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

	case NativeMsgUnpublishInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var unpubReq common.InteractionClassPayload
		if err := unpubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid unpublish interaction class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.Router().UnpublishInteractionClass(common.FederateHandle(ctx.federateID), unpubReq.ClassHandle); err != nil {
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
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

	case NativeMsgSubscribeInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var subReq common.SubscribeInteractionClassPayload
		if err := subReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid subscribe interaction class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		err := ctx.currentFed.Router().SubscribeInteractionClass(common.FederateHandle(ctx.federateID), subReq.ClassHandle, subReq.Active)
		if err != nil {
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

	case NativeMsgUnsubscribeInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var unsubReq common.InteractionClassPayload
		if err := unsubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid unsubscribe interaction class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.Router().UnsubscribeInteractionClass(common.FederateHandle(ctx.federateID), unsubReq.ClassHandle); err != nil {
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
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

