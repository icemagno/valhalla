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

	var fallbackSSZ []byte
	for _, fed := range f.GetAllFederates() {
		if fed.ID == ownerID || fed.Session == nil {
			continue
		}
		if sink, ok := fed.Session.(DomainEventSink); ok {
			if err := sink.SendDiscoverObject(obj.Handle, obj.ClassHandle, obj.Name); err != nil {
				logCRCDomain.Warn("failed to dispatch DiscoverObject to federate %d: %v", fed.ID, err)
			}
			continue
		}
		if fallbackSSZ == nil {
			fallbackSSZ, err = common.DiscoverObjectInstancePayload{
				ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
				ClassHandle:  common.ObjectClassHandle(obj.ClassHandle),
				ObjectName:   obj.Name,
			}.MarshalBinary()
			if err != nil {
				logCRCDomain.Error("failed to marshal DiscoverObjectInstancePayload for obj %d: %v", obj.Handle, err)
				return obj, nil
			}
		}
		if err := fed.Session.SendNotification(common.NativeMsgDiscoverObject, fallbackSSZ); err != nil {
			logCRCDomain.Warn("failed to dispatch DiscoverObject to federate %d: %v", fed.ID, err)
		}
	}
	return obj, nil
}

// DispatchCatchUpDiscovery sends DiscoverObject notifications for all existing instances
// matching the subscribed class hierarchy (unified across Native, Portico, and Web).
func (f *FederationExecution) DispatchCatchUpDiscovery(session Session, classHandle common.ObjectClassHandle) error {
	if session == nil || f.OMT() == nil {
		return nil
	}
	sink, hasSink := session.(DomainEventSink)
	for _, obj := range f.GetAllObjects() {
		if !f.OMT().IsSubclassOfObject(common.ObjectClassHandle(obj.ClassHandle), classHandle) {
			continue
		}
		if hasSink {
			if err := sink.SendDiscoverObject(obj.Handle, obj.ClassHandle, obj.Name); err != nil {
				logCRCDomain.Warn("failed to send catch-up DiscoverObject to federate %d: %v", session.FederateID(), err)
				return err
			}
		} else {
			discPayload, err := common.DiscoverObjectInstancePayload{
				ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
				ClassHandle:  common.ObjectClassHandle(obj.ClassHandle),
				ObjectName:   obj.Name,
			}.MarshalBinary()
			if err != nil {
				return err
			}
			if err := session.SendNotification(NativeMsgDiscoverObject, discPayload); err != nil {
				logCRCDomain.Warn("failed to send catch-up DiscoverObject to federate %d: %v", session.FederateID(), err)
				return err
			}
		}
	}
	return nil
}

// DeleteAndBroadcastObject validates delete privilege, deletes the instance, and broadcasts RemoveObjectInstance.
func (f *FederationExecution) DeleteAndBroadcastObject(objHandle int32, federateID int32, tag []byte) (*ObjectInstance, error) {
	obj, err := f.DeleteObjectInstance(objHandle, federateID, tag)
	if err != nil {
		return nil, err
	}

	var fallbackSSZ []byte
	for _, fed := range f.GetAllFederates() {
		if fed.ID == federateID || fed.Session == nil {
			continue
		}
		if sink, ok := fed.Session.(DomainEventSink); ok {
			if err := sink.SendRemoveObject(obj.Handle, tag); err != nil {
				logCRCDomain.Warn("failed to dispatch RemoveObjectInstance to federate %d: %v", fed.ID, err)
			}
			continue
		}
		if fallbackSSZ == nil {
			fallbackSSZ, err = common.RemoveObjectInstancePayload{
				ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
				Tag:          tag,
			}.MarshalBinary()
			if err != nil {
				logCRCDomain.Error("failed to marshal RemoveObjectInstancePayload for obj %d: %v", obj.Handle, err)
				return obj, nil
			}
		}
		if err := fed.Session.SendNotification(common.NativeMsgRemoveObjectInstance, fallbackSSZ); err != nil {
			logCRCDomain.Warn("failed to dispatch RemoveObjectInstance to federate %d: %v", fed.ID, err)
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

	// 5. Lazy SSZ payload (only built if TSO queuing or legacy non-DomainEventSink session needs it)
	var sszPayload []byte
	getSSZPayload := func() ([]byte, error) {
		if sszPayload != nil {
			return sszPayload, nil
		}
		var err error
		sszPayload, err = common.UpdateAttributesPayload{
			ObjectHandle: objHandle,
			Attributes:   attrs,
			Tag:          tag,
			HasTime:      hasTime,
			Time:         timestamp,
		}.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal UpdateAttributesPayload: %w", err)
		}
		return sszPayload, nil
	}

	// 6. Dispatch to subscribers (TSO or RO, single-pass per protocol)
	for subID := range targetSubs {
		subFed, ok := f.GetFederate(int32(subID))
		if !ok || subFed.Session == nil {
			continue
		}

		if hasTime && f.TimeManager() != nil && f.TimeManager().IsConstrained(subID) {
			ravPayload, err := getSSZPayload()
			if err != nil {
				return globalRoot, err
			}
			grants, err := f.TimeManager().EnqueueTSOMessage(core.TSOMessage{
				Sender:     common.FederateHandle(senderID),
				Receiver:   subID,
				Timestamp:  timestamp,
				MsgType:    NativeMsgReflectAttributes,
				Payload:    ravPayload,
				MerkleRoot: globalRoot,
			})
			if err != nil {
				logCRCDomain.Warn("failed to enqueue TSO message for federate %d: %v", subID, err)
			} else if len(grants) > 0 {
				dispatchTimeGrants(f, grants)
			}
		} else if sink, ok := subFed.Session.(DomainEventSink); ok {
			if err := sink.SendReflectAttributes(int32(objHandle), attrs, tag, hasTime, timestamp, globalRoot); err != nil {
				logCRCDomain.Warn("failed to dispatch ReflectAttributes to federate %d: %v", subID, err)
			}
		} else {
			ravPayload, err := getSSZPayload()
			if err != nil {
				return globalRoot, err
			}
			if err := subFed.Session.SendNotificationWithMerkle(NativeMsgReflectAttributes, ravPayload, globalRoot); err != nil {
				logCRCDomain.Warn("failed to dispatch ReflectAttributes to federate %d: %v", subID, err)
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

	var sszPayload []byte
	getSSZPayload := func() ([]byte, error) {
		if sszPayload != nil {
			return sszPayload, nil
		}
		var err error
		sszPayload, err = common.SendInteractionPayload{
			ClassHandle: classHandle,
			Parameters:  params,
			Tag:         tag,
			HasTime:     hasTime,
			Time:        timestamp,
		}.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal SendInteractionPayload: %w", err)
		}
		return sszPayload, nil
	}

	for _, subID := range subs {
		if subID == common.FederateHandle(senderID) {
			continue
		}
		subFed, ok := f.GetFederate(int32(subID))
		if !ok || subFed.Session == nil {
			continue
		}

		if hasTime && f.TimeManager() != nil && f.TimeManager().IsConstrained(subID) {
			payload, err := getSSZPayload()
			if err != nil {
				return err
			}
			grants, err := f.TimeManager().EnqueueTSOMessage(core.TSOMessage{
				Sender:    common.FederateHandle(senderID),
				Receiver:  subID,
				Timestamp: timestamp,
				MsgType:   NativeMsgReceiveInteraction,
				Payload:   payload,
			})
			if err != nil {
				logCRCDomain.Warn("failed to enqueue TSO interaction for federate %d: %v", subID, err)
			} else if len(grants) > 0 {
				dispatchTimeGrants(f, grants)
			}
		} else if sink, ok := subFed.Session.(DomainEventSink); ok {
			if err := sink.SendReceiveInteraction(int32(classHandle), params, tag, hasTime, timestamp); err != nil {
				logCRCDomain.Warn("failed to dispatch ReceiveInteraction to federate %d: %v", subID, err)
			}
		} else {
			payload, err := getSSZPayload()
			if err != nil {
				return err
			}
			if err := subFed.Session.SendNotification(NativeMsgReceiveInteraction, payload); err != nil {
				logCRCDomain.Warn("failed to dispatch ReceiveInteraction to federate %d: %v", subID, err)
			}
		}
	}
	return nil
}

// BroadcastSyncPointAnnounce announces a registered synchronization point to all federates.
func (f *FederationExecution) BroadcastSyncPointAnnounce(label string, tag []byte) error {
	var fallbackSSZ []byte
	for _, fed := range f.GetAllFederates() {
		if fed.Session == nil {
			continue
		}
		if sink, ok := fed.Session.(DomainEventSink); ok {
			if err := sink.SendAnnounceSyncPoint(label, tag); err != nil {
				logCRCDomain.Warn("failed to dispatch AnnounceSyncPoint to federate %d: %v", fed.ID, err)
			}
			continue
		}
		if fallbackSSZ == nil {
			var err error
			fallbackSSZ, err = common.RegisterSyncPointPayload{Label: label, Tag: tag}.MarshalBinary()
			if err != nil {
				return err
			}
		}
		if err := fed.Session.SendNotification(NativeMsgAnnounceSyncPoint, fallbackSSZ); err != nil {
			logCRCDomain.Warn("failed to dispatch AnnounceSyncPoint to federate %d: %v", fed.ID, err)
		}
	}
	return nil
}

// BroadcastFederationSynchronized notifies all federates that a sync point has been achieved.
func (f *FederationExecution) BroadcastFederationSynchronized(label string) error {
	var fallbackSSZ []byte
	for _, fed := range f.GetAllFederates() {
		if fed.Session == nil {
			continue
		}
		if sink, ok := fed.Session.(DomainEventSink); ok {
			if err := sink.SendFederationSynchronized(label); err != nil {
				logCRCDomain.Warn("failed to dispatch FederationSynchronized to federate %d: %v", fed.ID, err)
			}
			continue
		}
		if fallbackSSZ == nil {
			var err error
			fallbackSSZ, err = common.SyncPointLabelPayload{Label: label}.MarshalBinary()
			if err != nil {
				return err
			}
		}
		if err := fed.Session.SendNotification(NativeMsgFederationSynchronized, fallbackSSZ); err != nil {
			logCRCDomain.Warn("failed to dispatch FederationSynchronized to federate %d: %v", fed.ID, err)
		}
	}
	return nil
}
