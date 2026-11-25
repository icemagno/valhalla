package common

import "errors"

// Canonical normative IEEE 1516-2025 (HLA-4) errors
var (
	// Federation Management
	ErrFederationExecutionAlreadyExists = errors.New("hla: federation execution already exists")
	ErrFederationExecutionDoesNotExist  = errors.New("hla: federation execution does not exist")
	ErrFederatesCurrentlyJoined         = errors.New("hla: federates currently joined")
	ErrFederateNameAlreadyInUse         = errors.New("hla: federate name already in use")
	ErrFederateNotExecutionMember       = errors.New("hla: federate not execution member")
	ErrFederateNotFound                 = errors.New("hla: federate not found")

	// Synchronization Points
	ErrSyncPointLabelNotUnique = errors.New("hla: synchronization point label not unique")
	ErrSyncPointNotFound      = errors.New("hla: synchronization point not found")

	// Object Management
	ErrObjectClassNotDefined     = errors.New("hla: object class not defined")
	ErrObjectInstanceNotKnown    = errors.New("hla: object instance not known")
	ErrObjectInstanceNameInUse   = errors.New("hla: object instance name in use")
	ErrDeletePrivilegeNotHeld    = errors.New("hla: delete privilege not held")
	ErrAttributeNotDefined       = errors.New("hla: attribute not defined")
	ErrAttributeNotOwned         = errors.New("hla: attribute not owned")
	ErrInteractionClassNotDefined = errors.New("hla: interaction class not defined")

	// Time Management
	ErrInvalidLogicalTime              = errors.New("hla: invalid logical time")
	ErrInvalidLookahead                = errors.New("hla: invalid lookahead")
	ErrInTimeAdvancingState            = errors.New("hla: in time advancing state")
	ErrTimeAdvanceAlreadyInProgress    = errors.New("hla: time advance already in progress")
	ErrTimeRegulationAlreadyEnabled    = errors.New("hla: time regulation already enabled")
	ErrTimeRegulationNotEnabled        = errors.New("hla: time regulation not enabled")
	ErrTimeConstrainedAlreadyEnabled   = errors.New("hla: time constrained already enabled")
	ErrTimeConstrainedNotEnabled       = errors.New("hla: time constrained not enabled")
	ErrLogicalTimeAlreadyPassed        = errors.New("hla: logical time already passed")
	ErrRequestForTimeAdvancePending    = errors.New("hla: request for time advance pending")
)
