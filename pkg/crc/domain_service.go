package crc

import (
	"errors"
	"fmt"
	"sort"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

// dispatchTimeGrants delivers queued TSO messages and TimeAdvanceGrant notifications
// using the protocol-agnostic DomainEventSink interface whenever supported.
func dispatchTimeGrants(currentFed *FederationExecution, grants []core.TimeGrantEvent) {
	for _, grant := range grants {
		fed, ok := currentFed.GetFederate(int32(grant.Federate))
		if !ok || fed.Session == nil {
			continue
		}
		// Deliver any queued TSO messages first in strictly non-decreasing time order
		sink, hasSink := fed.Session.(DomainEventSink)
		for _, msg := range grant.Messages {
			switch msg.Kind {
			case core.TSOEventReflectAttributes:
				if hasSink {
					if err := sink.SendReflectAttributes(msg.ObjectHandle, msg.Attributes, msg.Tag, true, msg.Timestamp, msg.MerkleRoot); err != nil {
						logCRCDomain.Warn("failed to deliver TSO ReflectAttributes to federate %d: %v", grant.Federate, err)
					}
					continue
				}
				payload, err := common.UpdateAttributesPayload{
					ObjectHandle: common.ObjectInstanceHandle(msg.ObjectHandle),
					Attributes:   msg.Attributes,
					Tag:          msg.Tag,
					HasTime:      true,
					Time:         msg.Timestamp,
				}.MarshalBinary()
				if err != nil {
					logCRCDomain.Error("failed to marshal TSO ReflectAttributes: %v", err)
					continue
				}
				if err := fed.Session.SendNotificationWithMerkle(NativeMsgReflectAttributes, payload, msg.MerkleRoot); err != nil {
					logCRCDomain.Warn("failed to deliver TSO ReflectAttributes to federate %d: %v", grant.Federate, err)
				}
			case core.TSOEventReceiveInteraction:
				if hasSink {
					if err := sink.SendReceiveInteraction(msg.ClassHandle, msg.Parameters, msg.Tag, true, msg.Timestamp); err != nil {
						logCRCDomain.Warn("failed to deliver TSO ReceiveInteraction to federate %d: %v", grant.Federate, err)
					}
					continue
				}
				payload, err := common.SendInteractionPayload{
					ClassHandle: common.InteractionClassHandle(msg.ClassHandle),
					Parameters:  msg.Parameters,
					Tag:         msg.Tag,
					HasTime:     true,
					Time:        msg.Timestamp,
				}.MarshalBinary()
				if err != nil {
					logCRCDomain.Error("failed to marshal TSO ReceiveInteraction: %v", err)
					continue
				}
				if err := fed.Session.SendNotification(NativeMsgReceiveInteraction, payload); err != nil {
					logCRCDomain.Warn("failed to deliver TSO ReceiveInteraction to federate %d: %v", grant.Federate, err)
				}
			default:
				if err := fed.Session.SendNotificationWithMerkle(msg.MsgType, msg.Payload, msg.MerkleRoot); err != nil {
					logCRCDomain.Warn("failed to deliver TSO message %d to federate %d: %v", msg.MsgType, grant.Federate, err)
				}
			}
		}
		// Deliver TimeAdvanceGrant via DomainEventSink when available (single-pass per protocol)
		if hasSink {
			if err := sink.SendTimeAdvanceGrant(grant.Time); err != nil {
				logCRCDomain.Warn("failed to deliver TimeAdvanceGrant to federate %d: %v", grant.Federate, err)
			}
			continue
		}
		grantPayload, err := common.TimeFloat64Payload{Time: grant.Time}.MarshalBinary()
		if err != nil {
			logCRCDomain.Error("failed to marshal TimeFloat64Payload: %v", err)
			continue
		}
		if err := fed.Session.SendNotification(NativeMsgTimeAdvanceGrant, grantPayload); err != nil {
			logCRCDomain.Warn("failed to deliver TimeAdvanceGrant to federate %d: %v", grant.Federate, err)
		}
	}
}

// RegisterAndBroadcastObject validates publication, registers an object instance,
// and dispatches DiscoverObject strictly to subscribers of the object class hierarchy.
func (f *FederationExecution) RegisterAndBroadcastObject(classHandle int32, name string, ownerID int32) (*ObjectInstance, error) {
	if f.Router() != nil && f.OMT() != nil {
		if _, ok := f.OMT().GetObjectClass(common.ObjectClassHandle(classHandle)); ok {
			if !f.Router().IsObjectClassPublished(common.FederateHandle(ownerID), common.ObjectClassHandle(classHandle)) {
				return nil, core.ErrNotPublished
			}
		}
	}

	obj, err := f.RegisterObjectInstance(classHandle, name, ownerID)
	if err != nil {
		return nil, err
	}

	var targetIDs []common.FederateHandle
	if f.Router() != nil && f.OMT() != nil {
		if _, ok := f.OMT().GetObjectClass(common.ObjectClassHandle(obj.ClassHandle)); ok {
			targetIDs = f.Router().GetObjectClassSubscribers(common.ObjectClassHandle(obj.ClassHandle))
		}
	}

	var fallbackSSZ []byte
	for _, subID := range targetIDs {
		if int32(subID) == ownerID {
			continue
		}
		fed, ok := f.GetFederate(int32(subID))
		if !ok || fed.Session == nil {
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

// DeleteAndBroadcastObject validates delete privilege, deletes the instance,
// and dispatches RemoveObjectInstance strictly to subscribers of the object class hierarchy.
func (f *FederationExecution) DeleteAndBroadcastObject(objHandle int32, federateID int32, tag []byte) (*ObjectInstance, error) {
	obj, err := f.DeleteObjectInstance(objHandle, federateID, tag)
	if err != nil {
		return nil, err
	}

	var targetIDs []common.FederateHandle
	if f.Router() != nil && f.OMT() != nil {
		if _, ok := f.OMT().GetObjectClass(common.ObjectClassHandle(obj.ClassHandle)); ok {
			targetIDs = f.Router().GetObjectClassSubscribers(common.ObjectClassHandle(obj.ClassHandle))
		}
	}

	var fallbackSSZ []byte
	for _, subID := range targetIDs {
		if int32(subID) == federateID {
			continue
		}
		fed, ok := f.GetFederate(int32(subID))
		if !ok || fed.Session == nil {
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

	// 1. Verify attribute ownership (owned by sender, and not divested/unowned) and publication (IEEE 1516-2025)
	if f.ownershipMgr != nil {
		for attrHandle := range attrs {
			owner, isOwned := f.ownershipMgr.QueryAttributeOwnership(objHandle, attrHandle)
			if isOwned {
				if owner != common.FederateHandle(senderID) {
					return [32]byte{}, core.ErrNotAttributeOwner
				}
			} else if f.ownershipMgr.IsAttributeDivested(objHandle, attrHandle) {
				return [32]byte{}, core.ErrNotAttributeOwner
			}
		}
	}
	if f.router != nil {
		for attrHandle := range attrs {
			if !f.router.IsAttributePublished(common.FederateHandle(senderID), common.ObjectClassHandle(obj.ClassHandle), attrHandle) {
				return [32]byte{}, core.ErrNotPublished
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

	// 6. Dispatch to subscribers in strictly deterministic ascending handle order
	orderedSubs := make([]common.FederateHandle, 0, len(targetSubs))
	for subID := range targetSubs {
		orderedSubs = append(orderedSubs, subID)
	}
	sort.Slice(orderedSubs, func(i, j int) bool { return orderedSubs[i] < orderedSubs[j] })

	for _, subID := range orderedSubs {
		subFed, ok := f.GetFederate(int32(subID))
		if !ok || subFed.Session == nil {
			continue
		}

		if hasTime && f.TimeManager() != nil && f.TimeManager().IsConstrained(subID) {
			attrsClone := make(map[common.AttributeHandle][]byte, len(attrs))
			for k, v := range attrs {
				cp := make([]byte, len(v))
				copy(cp, v)
				attrsClone[k] = cp
			}
			var tagClone []byte
			if len(tag) > 0 {
				tagClone = make([]byte, len(tag))
				copy(tagClone, tag)
			}
			grants, err := f.TimeManager().EnqueueTSOMessage(core.TSOMessage{
				Sender:       common.FederateHandle(senderID),
				Receiver:     subID,
				Timestamp:    timestamp,
				Kind:         core.TSOEventReflectAttributes,
				ObjectHandle: int32(objHandle),
				Attributes:   attrsClone,
				Tag:          tagClone,
				MerkleRoot:   globalRoot,
			})
			if err != nil {
				logCRCDomain.Warn("failed to enqueue TSO message for federate %d: %v", subID, err)
				if errors.Is(err, common.ErrInvalidLogicalTime) || errors.Is(err, core.ErrTSOQueueFull) {
					return globalRoot, err
				}
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
	if f.Router() != nil && f.OMT() != nil {
		if _, ok := f.OMT().GetInteractionClass(classHandle); ok {
			if !f.Router().IsInteractionClassPublished(common.FederateHandle(senderID), classHandle) {
				return core.ErrNotPublished
			}
		}
	}
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
			paramsClone := make(map[common.ParameterHandle][]byte, len(params))
			for k, v := range params {
				cp := make([]byte, len(v))
				copy(cp, v)
				paramsClone[k] = cp
			}
			var tagClone []byte
			if len(tag) > 0 {
				tagClone = make([]byte, len(tag))
				copy(tagClone, tag)
			}
			grants, err := f.TimeManager().EnqueueTSOMessage(core.TSOMessage{
				Sender:      common.FederateHandle(senderID),
				Receiver:    subID,
				Timestamp:   timestamp,
				Kind:        core.TSOEventReceiveInteraction,
				ClassHandle: int32(classHandle),
				Parameters:  paramsClone,
				Tag:         tagClone,
			})
			if err != nil {
				logCRCDomain.Warn("failed to enqueue TSO interaction for federate %d: %v", subID, err)
				if errors.Is(err, common.ErrInvalidLogicalTime) || errors.Is(err, core.ErrTSOQueueFull) {
					return err
				}
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
