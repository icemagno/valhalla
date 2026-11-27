package lrc

import (
	"fmt"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

// UnconditionalAttributeOwnershipDivestiture unconditionally divests ownership of the specified attributes
func (a *RTIambassadorImpl) UnconditionalAttributeOwnershipDivestiture(objectHandle int32, attributes []int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrs := make([]uint32, len(attributes))
	for i, h := range attributes {
		attrs[i] = uint32(h)
	}

	req := common.DivestOwnershipPayload{
		ObjectHandle: uint32(objectHandle),
		Attributes:   attrs,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgDivestOwnership, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// AcquireAttributeOwnership acquires ownership of unowned or divested attributes
func (a *RTIambassadorImpl) AcquireAttributeOwnership(objectHandle int32, attributes []int32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrs := make([]uint32, len(attributes))
	for i, h := range attributes {
		attrs[i] = uint32(h)
	}

	req := common.AcquireOwnershipPayload{
		ObjectHandle: uint32(objectHandle),
		Attributes:   attrs,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgAcquireOwnership, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// QueryAttributeOwnership queries the current owner of an object instance attribute
func (a *RTIambassadorImpl) QueryAttributeOwnership(objectHandle int32, attributeHandle int32) (int32, bool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return 0, false, ErrNotConnected
	}
	if a.federateID == 0 {
		return 0, false, ErrFederateNotExecutionMember
	}

	req := common.QueryOwnershipPayload{
		ObjectHandle:    uint32(objectHandle),
		AttributeHandle: uint32(attributeHandle),
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return 0, false, err
	}

	resp, err := a.client.SendRequest(common.NativeMsgQueryAttributeOwnership, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return 0, false, err
	}
	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return 0, false, common.AsCanonicalError(errPayload.Message)
		}
		return 0, false, ErrInvalidResponse
	}
	if resp.Header.MsgType != common.NativeMsgAttributeOwnershipResponse {
		return 0, false, ErrInvalidResponse
	}

	var respPayload common.QueryOwnershipResponsePayload
	if err := respPayload.UnmarshalBinary(resp.Payload); err != nil {
		return 0, false, err
	}
	return int32(respPayload.OwnerHandle), respPayload.IsOwned, nil
}

// CreateRegion creates a spatial region for Data Distribution Management (DDM)
func (a *RTIambassadorImpl) CreateRegion(dimensions []int32) (uint32, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return 0, ErrNotConnected
	}
	if a.federateID == 0 {
		return 0, ErrFederateNotExecutionMember
	}

	dims := make([]uint32, len(dimensions))
	for i, d := range dimensions {
		dims[i] = uint32(d)
	}

	req := common.CreateRegionPayload{
		Dimensions: dims,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return 0, err
	}

	resp, err := a.client.SendRequest(common.NativeMsgCreateRegion, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return 0, err
	}
	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return 0, common.AsCanonicalError(errPayload.Message)
		}
		return 0, ErrInvalidResponse
	}
	if resp.Header.MsgType != common.NativeMsgRegionCreated {
		return 0, ErrInvalidResponse
	}

	var respPayload common.CreateRegionResponsePayload
	if err := respPayload.UnmarshalBinary(resp.Payload); err != nil {
		return 0, err
	}
	return respPayload.RegionHandle, nil
}

// SetRangeBounds sets discrete range bounds on a region dimension
func (a *RTIambassadorImpl) SetRangeBounds(regionHandle uint32, dimension int32, lowerBound, upperBound uint32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	req := common.SetRangeBoundsPayload{
		RegionHandle:    regionHandle,
		DimensionHandle: uint32(dimension),
		LowerBound:      lowerBound,
		UpperBound:      upperBound,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgSetRangeBounds, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// SetMetricBounds sets continuous ECEF metric bounds in meters for a DDM spatial region
func (a *RTIambassadorImpl) SetMetricBounds(regionHandle uint32, minX, maxX, minY, maxY, minZ, maxZ float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	req := common.SetMetricBoundsPayload{
		RegionHandle: regionHandle,
		MinX:         minX,
		MaxX:         maxX,
		MinY:         minY,
		MaxY:         maxY,
		MinZ:         minZ,
		MaxZ:         maxZ,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgSetMetricBounds, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// SubscribeObjectClassAttributesWithRegion subscribes to class attributes bound to a DDM spatial region
func (a *RTIambassadorImpl) SubscribeObjectClassAttributesWithRegion(objectClass int32, attributes []int32, regionHandle uint32) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrs := make([]uint32, len(attributes))
	for i, h := range attributes {
		attrs[i] = uint32(h)
	}

	req := common.SubscribeWithRegionPayload{
		ObjectClass:  uint32(objectClass),
		Attributes:   attrs,
		RegionHandle: regionHandle,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgSubscribeWithRegion, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// UpdateAttributeValuesWithSpatial dispatches updated attribute values with metric (ECEF) spatial coordinates
func (a *RTIambassadorImpl) UpdateAttributeValuesWithSpatial(objectHandle int32, attributes map[int32][]byte, x, y, z float64, tag []byte) error {
	a.mu.RLock()
	client := a.client
	fedID := a.federationID
	federateID := a.federateID
	fedAmb := a.fedAmb
	stateTree := a.stateTree
	a.mu.RUnlock()

	if client == nil || client.IsClosed() {
		return ErrNotConnected
	}
	if federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	attrMap := make(map[common.AttributeHandle][]byte, len(attributes))
	for k, v := range attributes {
		attrMap[common.AttributeHandle(k)] = v
	}

	req := common.UpdateAttributesSpatialPayload{
		ObjectHandle: common.ObjectInstanceHandle(objectHandle),
		Attributes:   attrMap,
		Tag:          tag,
		X:            x,
		Y:            y,
		Z:            z,
	}
	payload, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgUpdateAttributesSpatial, uint32(fedID), uint32(federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	if err := checkNativeResponse(resp); err != nil {
		return err
	}

	for k, v := range attrMap {
		stateTree.UpdateEntityAttribute(common.ObjectInstanceHandle(objectHandle), k, v)
	}
	if (resp.Header.Flags & common.FlagNativeMerkle) != 0 {
		localRoot := stateTree.GetGlobalRoot()
		if localRoot != resp.Header.MerkleRoot {
			desyncErr := fmt.Errorf("%w: CRC root %x != local root %x", ErrStateDesyncDetected, resp.Header.MerkleRoot, localRoot)
			a.desyncMu.Lock()
			a.lastDesyncErr = desyncErr
			a.desyncMu.Unlock()
			if desyncAmb, ok := fedAmb.(DesyncAwareFederateAmbassador); ok {
				desyncAmb.OnStateDesyncDetected(resp.Header.MerkleRoot, localRoot)
			}
		}
	}
	return nil
}

// GetLocalStateRoot returns the deterministic 32-byte Merkle root of the local LRC state
func (a *RTIambassadorImpl) GetLocalStateRoot() [32]byte {
	return a.stateTree.GetGlobalRoot()
}

// LastDesyncDetected returns the last state desynchronization error, if any
func (a *RTIambassadorImpl) LastDesyncDetected() error {
	a.desyncMu.RLock()
	defer a.desyncMu.RUnlock()
	return a.lastDesyncErr
}

// GetObjectMerkleProof requests a cryptographic Merkle inclusion proof for an object from the CRC
func (a *RTIambassadorImpl) GetObjectMerkleProof(objectHandle int32) (*common.MerkleProof, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return nil, ErrNotConnected
	}
	if a.federateID == 0 {
		return nil, ErrFederateNotExecutionMember
	}

	reqPayload, err := common.GetMerkleProofPayload{
		ObjectHandle: common.ObjectInstanceHandle(objectHandle),
	}.MarshalBinary()
	if err != nil {
		return nil, err
	}

	resp, err := a.client.SendRequest(common.NativeMsgGetMerkleProof, uint32(a.federationID), uint32(a.federateID), reqPayload, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return nil, common.AsCanonicalError(errPayload.Message)
		}
		return nil, ErrInvalidResponse
	}
	if resp.Header.MsgType != common.NativeMsgGetMerkleProofResp {
		return nil, ErrInvalidResponse
	}

	var proofResp common.MerkleProofResponsePayload
	if err := proofResp.UnmarshalBinary(resp.Payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merkle proof response: %w", err)
	}

	return &common.MerkleProof{
		LeafHash:   proofResp.LeafHash,
		AuditPath:  proofResp.AuditPath,
		LeafIndex:  proofResp.LeafIndex,
		GlobalRoot: proofResp.GlobalRoot,
	}, nil
}

// VerifyObjectInclusion mathematically validates a Merkle proof against a trusted expectedRoot
func (a *RTIambassadorImpl) VerifyObjectInclusion(proof *common.MerkleProof, expectedRoot [32]byte) bool {
	return common.VerifyMerkleProof(proof, expectedRoot)
}

