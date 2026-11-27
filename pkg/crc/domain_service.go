package crc

import (
	"fmt"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

// RegisterAndBroadcastObject registers an object instance and broadcasts DiscoverObject to all other federates.
func (f *FederationExecution) RegisterAndBroadcastObject(classHandle int32, name string, ownerID int32) (*ObjectInstance, error) {
	obj, err := f.RegisterObjectInstance(classHandle, name, ownerID)
	if err != nil {
		return nil, err
	}

	discPayload, err := common.DiscoverObjectInstancePayload{
		ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
		ClassHandle:  common.ObjectClassHandle(obj.ClassHandle),
		ObjectName:   obj.Name,
	}.MarshalBinary()
	if err != nil {
		logCRCNative.Error("failed to marshal DiscoverObjectInstancePayload for obj %d: %v", obj.Handle, err)
		return obj, nil
	}

	for _, fed := range f.GetAllFederates() {
		if fed.ID != ownerID && fed.Session != nil {
			if err := fed.Session.SendNotification(common.NativeMsgDiscoverObject, discPayload); err != nil {
				logCRCNative.Warn("failed to dispatch DiscoverObject to federate %d: %v", fed.ID, err)
			}
		}
	}
	return obj, nil
}

// DeleteAndBroadcastObject validates delete privilege, deletes the instance, and broadcasts RemoveObjectInstance.
func (f *FederationExecution) DeleteAndBroadcastObject(objHandle int32, federateID int32, tag []byte) (*ObjectInstance, error) {
	obj, err := f.DeleteObjectInstance(objHandle, federateID, tag)
	if err != nil {
		return nil, err
	}

	removePayload, err := common.RemoveObjectInstancePayload{
		ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
		Tag:          tag,
	}.MarshalBinary()
	if err != nil {
		logCRCNative.Error("failed to marshal RemoveObjectInstancePayload for obj %d: %v", obj.Handle, err)
		return obj, nil
	}

	for _, fed := range f.GetAllFederates() {
		if fed.ID != federateID && fed.Session != nil {
			if err := fed.Session.SendNotification(common.NativeMsgRemoveObjectInstance, removePayload); err != nil {
				logCRCNative.Warn("failed to dispatch RemoveObjectInstance to federate %d: %v", fed.ID, err)
			}
		}
	}
	return obj, nil
}

// UpdateAndRouteAttributes updates object attributes, verifies ownership, updates Merkle tree, and routes reflections.
func (f *FederationExecution) UpdateAndRouteAttributes(
	senderID int32,
	objHandle common.ObjectInstanceHandle,
	attrs map[common.AttributeHandle][]byte,
	tag []byte,
	hasTime bool,
	timestamp float64,
	mortonCode *uint64,
	clientMerkleRoot [32]byte,
) ([32]byte, error) {
	obj, exists := f.GetObjectInstance(int32(objHandle))
	if !exists {
		return [32]byte{}, common.ErrObjectInstanceNotKnown
	}

	// 1. Verify attribute ownership
	if f.ownershipMgr != nil {
		for attrHandle := range attrs {
			if owner, isOwned := f.ownershipMgr.QueryAttributeOwnership(objHandle, attrHandle); isOwned && owner != common.FederateHandle(senderID) {
				return [32]byte{}, core.ErrNotAttributeOwner
			}
		}
	}

	// 2. Update object attributes and Merkle state
	attrsToStore := make(map[int32][]byte, len(attrs))
	for k, v := range attrs {
		attrsToStore[int32(k)] = v
	}
	if err := f.UpdateObjectAttributes(int32(objHandle), attrsToStore, clientMerkleRoot); err != nil {
		return [32]byte{}, err
	}

	// 3. Compute global root if Merkle audit is enabled
	var globalRoot [32]byte
	if f.IsMerkleAuditEnabled() {
		globalRoot = f.StateTree().GetGlobalRoot()
	}

	// 4. Compute target subscribers
	targetSubs := make(map[common.FederateHandle]bool)
	if mortonCode != nil {
		for attrHandle := range attrs {
			subs := f.Router().GetAttributeSubscribersInRegion(common.ObjectClassHandle(obj.ClassHandle), attrHandle, *mortonCode)
			for _, sub := range subs {
				if sub != common.FederateHandle(senderID) {
					targetSubs[sub] = true
				}
			}
		}
	} else {
		for attrHandle := range attrs {
			subs := f.Router().GetAttributeSubscribers(common.ObjectClassHandle(obj.ClassHandle), attrHandle)
			for _, sub := range subs {
				if sub != common.FederateHandle(senderID) {
					targetSubs[sub] = true
				}
			}
		}
	}

	// 5. Build reflect payload
	ravPayload, err := common.UpdateAttributesPayload{
		ObjectHandle: objHandle,
		Attributes:   attrs,
		Tag:          tag,
		HasTime:      hasTime,
		Time:         timestamp,
	}.MarshalBinary()
	if err != nil {
		return globalRoot, fmt.Errorf("failed to marshal UpdateAttributesPayload: %w", err)
	}

	// 6. Dispatch to subscribers (TSO or RO)
	for subID := range targetSubs {
		subFed, ok := f.GetFederate(int32(subID))
		if !ok || subFed.Session == nil {
			continue
		}

		if hasTime && subFed.IsConstrained {
			grants, err := f.TimeManager().EnqueueTSOMessage(core.TSOMessage{
				Sender:     common.FederateHandle(senderID),
				Receiver:   subID,
				Timestamp:  timestamp,
				MsgType:    NativeMsgReflectAttributes,
				Payload:    ravPayload,
				MerkleRoot: globalRoot,
			})
			if err != nil {
				logCRCNative.Warn("failed to enqueue TSO message for federate %d: %v", subID, err)
			} else if len(grants) > 0 {
				dispatchTimeGrants(f, grants)
			}
		} else {
			if err := subFed.Session.SendNotificationWithMerkle(NativeMsgReflectAttributes, ravPayload, globalRoot); err != nil {
				logCRCNative.Warn("failed to dispatch ReflectAttributes to federate %d: %v", subID, err)
			}
		}
	}

	return globalRoot, nil
}

// RouteInteraction routes an interaction to all subscribed federates (either TSO or RO).
func (f *FederationExecution) RouteInteraction(
	senderID int32,
	classHandle common.InteractionClassHandle,
	params map[common.ParameterHandle][]byte,
	tag []byte,
	hasTime bool,
	timestamp float64,
) error {
	subs := f.Router().GetInteractionSubscribers(classHandle)

	payload, err := common.SendInteractionPayload{
		ClassHandle: classHandle,
		Parameters:  params,
		Tag:         tag,
		HasTime:     hasTime,
		Time:        timestamp,
	}.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal SendInteractionPayload: %w", err)
	}

	for _, subID := range subs {
		if subID == common.FederateHandle(senderID) {
			continue
		}
		subFed, ok := f.GetFederate(int32(subID))
		if !ok || subFed.Session == nil {
			continue
		}

		if hasTime && subFed.IsConstrained {
			grants, err := f.TimeManager().EnqueueTSOMessage(core.TSOMessage{
				Sender:    common.FederateHandle(senderID),
				Receiver:  subID,
				Timestamp: timestamp,
				MsgType:   NativeMsgReceiveInteraction,
				Payload:   payload,
			})
			if err != nil {
				logCRCNative.Warn("failed to enqueue TSO interaction for federate %d: %v", subID, err)
			} else if len(grants) > 0 {
				dispatchTimeGrants(f, grants)
			}
		} else {
			if err := subFed.Session.SendNotification(NativeMsgReceiveInteraction, payload); err != nil {
				logCRCNative.Warn("failed to dispatch ReceiveInteraction to federate %d: %v", subID, err)
			}
		}
	}
	return nil
}

// BroadcastSyncPointAnnounce announces a registered synchronization point to all federates.
func (f *FederationExecution) BroadcastSyncPointAnnounce(label string, tag []byte) error {
	announcePayload, err := common.RegisterSyncPointPayload{Label: label, Tag: tag}.MarshalBinary()
	if err != nil {
		return err
	}
	for _, fed := range f.GetAllFederates() {
		if fed.Session != nil {
			if err := fed.Session.SendNotification(NativeMsgAnnounceSyncPoint, announcePayload); err != nil {
				logCRCNative.Warn("failed to dispatch AnnounceSyncPoint to federate %d: %v", fed.ID, err)
			}
		}
	}
	return nil
}

// BroadcastFederationSynchronized notifies all federates that a sync point has been achieved.
func (f *FederationExecution) BroadcastFederationSynchronized(label string) error {
	syncNotif, err := common.SyncPointLabelPayload{Label: label}.MarshalBinary()
	if err != nil {
		return err
	}
	for _, fed := range f.GetAllFederates() {
		if fed.Session != nil {
			if err := fed.Session.SendNotification(NativeMsgFederationSynchronized, syncNotif); err != nil {
				logCRCNative.Warn("failed to dispatch FederationSynchronized to federate %d: %v", fed.ID, err)
			}
		}
	}
	return nil
}
