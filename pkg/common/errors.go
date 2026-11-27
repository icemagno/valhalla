package common

import "errors"

// Canonical normative IEEE 1516-2025 (HLA-4) errors
var (
	// Federation Management
	ErrFederationExecutionAlreadyExists = errors.New("hla: federation execution already exists")
	ErrFederationExecutionDoesNotExist  = errors.New("hla: federation execution does not exist")
	ErrFederatesCurrentlyJoined         = errors.New("hla: federates currently joined")
	ErrFederateNameAlreadyInUse         = errors.New("hla: federate name already in use")
	ErrFederateAlreadyExecutionMember   = errors.New("hla: federate already execution member")
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

	// OMT & Metamodel (FOM / MIM)
	ErrClassNotFound        = errors.New("hla omt: class not found in FOM registry")
	ErrAttributeNotFound    = errors.New("hla omt: attribute not found in class hierarchy")
	ErrInteractionNotFound  = errors.New("hla omt: interaction class not found in FOM registry")
	ErrParameterNotFound    = errors.New("hla omt: parameter not found in interaction hierarchy")
	ErrDuplicateClass       = errors.New("hla omt: class name already registered")
	ErrDuplicateAttribute   = errors.New("hla omt: attribute name already exists in class")
	ErrDuplicateInteraction = errors.New("hla omt: interaction name already registered")
	ErrDuplicateParameter   = errors.New("hla omt: parameter name already exists in interaction")
	ErrInvalidParentClass   = errors.New("hla omt: invalid or non-existent parent class")
	ErrAmbiguousClass       = errors.New("hla omt: ambiguous class name matches multiple classes; use fully qualified name")
	ErrAmbiguousInteraction = errors.New("hla omt: ambiguous interaction name matches multiple classes; use fully qualified name")
)

// AsCanonicalError resolves an error message back to its canonical typed sentinel error
func AsCanonicalError(msg string) error {
	switch msg {
	case ErrFederationExecutionAlreadyExists.Error():
		return ErrFederationExecutionAlreadyExists
	case ErrFederationExecutionDoesNotExist.Error():
		return ErrFederationExecutionDoesNotExist
	case ErrFederateAlreadyExecutionMember.Error():
		return ErrFederateAlreadyExecutionMember
	case ErrFederatesCurrentlyJoined.Error():
		return ErrFederatesCurrentlyJoined
	case ErrFederateNameAlreadyInUse.Error():
		return ErrFederateNameAlreadyInUse
	case ErrFederateNotExecutionMember.Error():
		return ErrFederateNotExecutionMember
	case ErrFederateNotFound.Error():
		return ErrFederateNotFound
	case ErrSyncPointLabelNotUnique.Error():
		return ErrSyncPointLabelNotUnique
	case ErrSyncPointNotFound.Error():
		return ErrSyncPointNotFound
	case ErrObjectClassNotDefined.Error():
		return ErrObjectClassNotDefined
	case ErrObjectInstanceNotKnown.Error():
		return ErrObjectInstanceNotKnown
	case ErrObjectInstanceNameInUse.Error():
		return ErrObjectInstanceNameInUse
	case ErrDeletePrivilegeNotHeld.Error():
		return ErrDeletePrivilegeNotHeld
	case ErrAttributeNotDefined.Error():
		return ErrAttributeNotDefined
	case ErrAttributeNotOwned.Error():
		return ErrAttributeNotOwned
	case ErrInteractionClassNotDefined.Error():
		return ErrInteractionClassNotDefined
	case ErrInvalidLogicalTime.Error():
		return ErrInvalidLogicalTime
	case ErrInvalidLookahead.Error():
		return ErrInvalidLookahead
	case ErrInTimeAdvancingState.Error():
		return ErrInTimeAdvancingState
	case ErrTimeAdvanceAlreadyInProgress.Error():
		return ErrTimeAdvanceAlreadyInProgress
	case ErrTimeRegulationAlreadyEnabled.Error():
		return ErrTimeRegulationAlreadyEnabled
	case ErrTimeRegulationNotEnabled.Error():
		return ErrTimeRegulationNotEnabled
	case ErrTimeConstrainedAlreadyEnabled.Error():
		return ErrTimeConstrainedAlreadyEnabled
	case ErrTimeConstrainedNotEnabled.Error():
		return ErrTimeConstrainedNotEnabled
	case ErrLogicalTimeAlreadyPassed.Error():
		return ErrLogicalTimeAlreadyPassed
	case ErrRequestForTimeAdvancePending.Error():
		return ErrRequestForTimeAdvancePending
	case ErrClassNotFound.Error():
		return ErrClassNotFound
	case ErrAttributeNotFound.Error():
		return ErrAttributeNotFound
	case ErrInteractionNotFound.Error():
		return ErrInteractionNotFound
	case ErrParameterNotFound.Error():
		return ErrParameterNotFound
	case ErrDuplicateClass.Error():
		return ErrDuplicateClass
	case ErrDuplicateAttribute.Error():
		return ErrDuplicateAttribute
	case ErrDuplicateInteraction.Error():
		return ErrDuplicateInteraction
	case ErrDuplicateParameter.Error():
		return ErrDuplicateParameter
	case ErrInvalidParentClass.Error():
		return ErrInvalidParentClass
	case ErrAmbiguousClass.Error():
		return ErrAmbiguousClass
	case ErrAmbiguousInteraction.Error():
		return ErrAmbiguousInteraction
	default:
		return errors.New(msg)
	}
}
