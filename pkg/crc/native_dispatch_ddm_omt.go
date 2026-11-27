package crc

import (
	"errors"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

// handleDDMManagement handles IEEE 1516-2025 Clause 9 services
func (ctx *nativeContext) handleDDMManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgCreateRegion:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.CreateRegionPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid create region payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		regionHandle := ctx.currentFed.CreateRegion()
		respData, _ := common.CreateRegionResponsePayload{
			RegionHandle: regionHandle,
		}.MarshalBinary()

		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			PayloadLen: uint32(len(respData)),
			MsgType:    NativeMsgRegionCreated,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respData, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case NativeMsgSetRangeBounds:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.SetRangeBoundsPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid set range bounds payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.SetRegionRangeBounds(req.RegionHandle, req.DimensionHandle, req.LowerBound, req.UpperBound); err != nil {
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

	case NativeMsgSetMetricBounds:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.SetMetricBoundsPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid set metric bounds payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.SetRegionMetricBounds(req.RegionHandle, req.MinX, req.MaxX, req.MinY, req.MaxY, req.MinZ, req.MaxZ); err != nil {
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

	case NativeMsgSubscribeWithRegion:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.SubscribeWithRegionPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid subscribe with region payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		region, exists := ctx.currentFed.GetRegion(req.RegionHandle)
		if !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, errors.New("region handle not found")); err != nil {
				return true, err
			}
			return true, nil
		}
		attrs := convertToAttrHandles(req.Attributes)
		if err := ctx.currentFed.Router().SubscribeObjectClassWithRegion(common.FederateHandle(ctx.federateID), common.ObjectClassHandle(req.ObjectClass), attrs, region); err != nil {
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

	case NativeMsgUpdateAttributesSpatial:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.UpdateAttributesSpatialPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid spatial update attributes payload")); err != nil {
				return true, err
			}
			return true, nil
		}

		morton := core.EncodeECEF(req.X, req.Y, req.Z)
		globalRoot, err := ctx.currentFed.UpdateAndRouteAttributes(
			ctx.federateID,
			req.ObjectHandle,
			req.Attributes,
			req.Tag,
			false,
			0,
			&morton,
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
				logCRCNative.Warn("failed to update attributes with region for object %d: %v", req.ObjectHandle, err)
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
	}
	return false, nil
}

// handleMetadataManagement handles IEEE 1516-2025 Clause 10 / Valhalla support services
func (ctx *nativeContext) handleMetadataManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgHeartbeat:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		ctx.currentFed.TimeManager().Heartbeat(common.FederateHandle(ctx.federateID))
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

	case common.NativeMsgGetObjectClassHandle:
		if ctx.currentFed == nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var qReq common.GetObjectClassHandlePayload
		if err := qReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid get object class handle payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		cls, ok := ctx.currentFed.OMT().GetObjectClassByName(qReq.ClassName)
		if !ok {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, core.ErrClassNotFound); err != nil {
				return true, err
			}
			return true, nil
		}
		respPayload, _ := common.HandleResponsePayload{Handle: int32(cls.Handle)}.MarshalBinary()
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    common.NativeMsgGetObjectClassHandleResp,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			PayloadLen: uint32(len(respPayload)),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case common.NativeMsgGetAttributeHandle:
		if ctx.currentFed == nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var qReq common.GetAttributeHandlePayload
		if err := qReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid get attribute handle payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		attr, err := ctx.currentFed.OMT().ResolveAttributeByName(common.ObjectClassHandle(qReq.ClassHandle), qReq.AttributeName)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respPayload, _ := common.HandleResponsePayload{Handle: int32(attr.Handle)}.MarshalBinary()
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    common.NativeMsgGetAttributeHandleResp,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			PayloadLen: uint32(len(respPayload)),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case common.NativeMsgGetDimensionHandle:
		if ctx.currentFed == nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var qReq common.GetDimensionHandlePayload
		if err := qReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid get dimension handle payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		dim, ok := ctx.currentFed.OMT().GetDimensionByName(qReq.DimensionName)
		if !ok {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, errors.New("dimension not found")); err != nil {
				return true, err
			}
			return true, nil
		}
		respPayload, _ := common.HandleResponsePayload{Handle: int32(dim.Handle)}.MarshalBinary()
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    common.NativeMsgGetDimensionHandleResp,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			PayloadLen: uint32(len(respPayload)),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case common.NativeMsgGetMerkleProof:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var qReq common.GetMerkleProofPayload
		if err := qReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid get merkle proof payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		proof, err := ctx.currentFed.StateTree().GenerateProof(qReq.ObjectHandle)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respPayload, err := common.MerkleProofResponsePayload{
			ObjectHandle: qReq.ObjectHandle,
			LeafIndex:    proof.LeafIndex,
			GlobalRoot:   proof.GlobalRoot,
			LeafHash:     proof.LeafHash,
			AuditPath:    proof.AuditPath,
		}.MarshalBinary()
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 500, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      common.FlagNativeSSZ | common.FlagNativeMerkle,
			MsgType:    common.NativeMsgGetMerkleProofResp,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			PayloadLen: uint32(len(respPayload)),
			MerkleRoot: proof.GlobalRoot,
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}
