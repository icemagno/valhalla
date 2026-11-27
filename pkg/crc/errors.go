package crc

import "errors"

var (
	ErrFederationExecutionAlreadyExists = errors.New("hla crc: federation execution already exists")
	ErrFederationExecutionDoesNotExist  = errors.New("hla crc: federation execution does not exist")
	ErrFederatesCurrentlyJoined         = errors.New("hla crc: federates currently joined")
	ErrFederateNameAlreadyInUse         = errors.New("hla crc: federate name already in use")
	ErrFederateNotFound                 = errors.New("hla crc: federate not found in federation")
	ErrSyncPointLabelNotUnique          = errors.New("hla crc: synchronization point label not unique")
	ErrSyncPointNotFound                = errors.New("hla crc: synchronization point not found")
	ErrObjectInstanceNameNotUnique      = errors.New("hla crc: object instance name not unique")
	ErrMerkleRootMismatch               = errors.New("hla crc: client-supplied merkle root does not match authoritative object state")
)
