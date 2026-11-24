package lrc

import "errors"

var (
	ErrNotConnected                   = errors.New("hla lrc: federate not connected to rti")
	ErrAlreadyConnected               = errors.New("hla lrc: federate already connected to rti")
	ErrFederateNotExecutionMember     = errors.New("hla lrc: federate is not execution member")
	ErrFederateAlreadyExecutionMember = errors.New("hla lrc: federate already execution member")
	ErrConnectionClosed               = errors.New("hla lrc: transport connection closed")
	ErrRequestTimeout                 = errors.New("hla lrc: request timeout waiting for crc response")
	ErrInvalidResponse                = errors.New("hla lrc: invalid response from crc")
)
