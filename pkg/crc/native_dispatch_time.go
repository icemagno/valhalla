package crc

import (
	"errors"

	"github.com/icemagno/valhalla/pkg/common"
)

// handleTimeManagement handles IEEE 1516-2025 Clause 8 services
func (ctx *nativeContext) handleTimeManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgEnableTimeRegulation:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var timeReq common.TimeFloat64Payload
		if err := timeReq.UnmarshalBinary(payload); err != nil || timeReq.Time < 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, common.ErrInvalidLookahead); err != nil {
				return true, err
			}
			return true, nil
		}
		if _, exists := ctx.currentFed.GetFederate(ctx.federateID); !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, common.ErrFederateNotFound); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.TimeManager().EnableTimeRegulation(common.FederateHandle(ctx.federateID), timeReq.Time); err != nil {
			if errors.Is(err, common.ErrTimeRegulationAlreadyEnabled) {
				if err := sendNativeError(ctx.session, hdr.ReqID, 409, err); err != nil {
					return true, err
				}
			} else {
				if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
					return true, err
				}
			}
			return true, nil
		}
		status, _ := ctx.currentFed.TimeManager().GetStatus(common.FederateHandle(ctx.federateID))
		respData, err := common.TimeFloat64Payload{Time: status.CurrentTime}.MarshalBinary()
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
			PayloadLen: uint32(len(respData)),
			MsgType:    NativeMsgTimeRegulationEnabled,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respData, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Time regulation enabled for federate [%d], lookahead=%f", ctx.federateID, timeReq.Time)
		return true, nil

	case NativeMsgDisableTimeRegulation:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		grants, err := ctx.currentFed.TimeManager().DisableTimeRegulation(common.FederateHandle(ctx.federateID))
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
		dispatchTimeGrants(ctx.currentFed, grants)
		return true, nil

	case NativeMsgEnableTimeConstrained:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		if _, exists := ctx.currentFed.GetFederate(ctx.federateID); !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, common.ErrFederateNotFound); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.TimeManager().EnableTimeConstrained(common.FederateHandle(ctx.federateID)); err != nil {
			if errors.Is(err, common.ErrTimeConstrainedAlreadyEnabled) {
				if err := sendNativeError(ctx.session, hdr.ReqID, 409, err); err != nil {
					return true, err
				}
			} else {
				if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
					return true, err
				}
			}
			return true, nil
		}

		status, _ := ctx.currentFed.TimeManager().GetStatus(common.FederateHandle(ctx.federateID))
		respData, err := common.TimeFloat64Payload{Time: status.CurrentTime}.MarshalBinary()
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
			PayloadLen: uint32(len(respData)),
			MsgType:    NativeMsgTimeConstrainedEnabled,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respData, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Time constrained enabled for federate [%d]", ctx.federateID)
		return true, nil

	case NativeMsgDisableTimeConstrained:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		grants, err := ctx.currentFed.TimeManager().DisableTimeConstrained(common.FederateHandle(ctx.federateID))
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
		dispatchTimeGrants(ctx.currentFed, grants)
		return true, nil

	case NativeMsgTimeAdvanceRequest:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var tarReq common.TimeFloat64Payload
		if err := tarReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid time advance request")); err != nil {
				return true, err
			}
			return true, nil
		}
		grants, err := ctx.currentFed.TimeManager().TimeAdvanceRequest(common.FederateHandle(ctx.federateID), tarReq.Time)
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
		dispatchTimeGrants(ctx.currentFed, grants)
		return true, nil

	case NativeMsgNextMessageRequest:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var nmrReq common.TimeFloat64Payload
		if err := nmrReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid next message request")); err != nil {
				return true, err
			}
			return true, nil
		}
		grants, err := ctx.currentFed.TimeManager().NextMessageRequest(common.FederateHandle(ctx.federateID), nmrReq.Time)
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
		dispatchTimeGrants(ctx.currentFed, grants)
		return true, nil

	case NativeMsgFlushQueueRequest:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var flushReq common.TimeFloat64Payload
		if err := flushReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid flush queue request")); err != nil {
				return true, err
			}
			return true, nil
		}
		grants, err := ctx.currentFed.TimeManager().FlushQueueRequest(common.FederateHandle(ctx.federateID), flushReq.Time)
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
		dispatchTimeGrants(ctx.currentFed, grants)
		return true, nil
	}
	return false, nil
}

