// RTIexception.h - Official IEEE 1516e Exception Hierarchy
#ifndef RTI1516E_RTIEXCEPTION_H
#define RTI1516E_RTIEXCEPTION_H

#include <exception>
#include <string>

#if __cplusplus >= 201703L
#define RTI1516E_THROW(...)
#else
#define RTI1516E_THROW(...) throw(__VA_ARGS__)
#endif

namespace rti1516e {

class RTIexception : public std::exception {
public:
    explicit RTIexception(std::wstring message)
        : _wmsg(std::move(message)) {
        for (wchar_t wc : _wmsg) {
            _msg.push_back(static_cast<char>(wc < 128 ? wc : '?'));
        }
    }

    explicit RTIexception(std::string message)
        : _msg(std::move(message)) {
        for (char c : _msg) {
            _wmsg.push_back(static_cast<wchar_t>(static_cast<unsigned char>(c)));
        }
    }

    virtual ~RTIexception() throw() {}

    virtual const wchar_t* whatW() const throw() { return _wmsg.c_str(); }
    virtual const char* what() const throw() override { return _msg.c_str(); }

private:
    std::wstring _wmsg;
    std::string _msg;
};

#define DECLARE_RTI_EXCEPTION(Name) \
    class Name : public RTIexception { \
    public: \
        explicit Name(std::wstring message) : RTIexception(std::move(message)) {} \
        explicit Name(std::string message) : RTIexception(std::move(message)) {} \
        virtual ~Name() throw() {} \
    };

// Core
DECLARE_RTI_EXCEPTION(RTIinternalError)
DECLARE_RTI_EXCEPTION(NotConnected)
DECLARE_RTI_EXCEPTION(AlreadyConnected)
DECLARE_RTI_EXCEPTION(ConnectionFailed)
DECLARE_RTI_EXCEPTION(InvalidLocalSettingsDesignator)
DECLARE_RTI_EXCEPTION(UnsupportedCallbackModel)
DECLARE_RTI_EXCEPTION(CallNotAllowedFromWithinCallback)
DECLARE_RTI_EXCEPTION(FederateInternalError)

// Federation Management
DECLARE_RTI_EXCEPTION(FederateNotExecutionMember)
DECLARE_RTI_EXCEPTION(FederateAlreadyExecutionMember)
DECLARE_RTI_EXCEPTION(FederateIsExecutionMember)
DECLARE_RTI_EXCEPTION(FederationExecutionAlreadyExists)
DECLARE_RTI_EXCEPTION(FederationExecutionDoesNotExist)
DECLARE_RTI_EXCEPTION(FederatesCurrentlyJoined)
DECLARE_RTI_EXCEPTION(FederateNameAlreadyInUse)
DECLARE_RTI_EXCEPTION(CouldNotCreateLogicalTimeFactory)
DECLARE_RTI_EXCEPTION(InconsistentFDD)
DECLARE_RTI_EXCEPTION(ErrorReadingFDD)
DECLARE_RTI_EXCEPTION(CouldNotOpenFDD)

// OMT & Declarations
DECLARE_RTI_EXCEPTION(ObjectClassNotDefined)
DECLARE_RTI_EXCEPTION(ObjectClassNotPublished)
DECLARE_RTI_EXCEPTION(ObjectClassNotSubscribed)
DECLARE_RTI_EXCEPTION(AttributeNotDefined)
DECLARE_RTI_EXCEPTION(AttributeNotOwned)
DECLARE_RTI_EXCEPTION(ObjectInstanceNotKnown)
DECLARE_RTI_EXCEPTION(ObjectInstanceNameInUse)
DECLARE_RTI_EXCEPTION(ObjectInstanceNameNotReserved)
DECLARE_RTI_EXCEPTION(DeletePrivilegeNotHeld)
DECLARE_RTI_EXCEPTION(InteractionClassNotDefined)
DECLARE_RTI_EXCEPTION(InteractionClassNotPublished)
DECLARE_RTI_EXCEPTION(InteractionClassNotSubscribed)
DECLARE_RTI_EXCEPTION(NameNotFound)
DECLARE_RTI_EXCEPTION(InvalidObjectClassHandle)

// Ownership
DECLARE_RTI_EXCEPTION(OwnershipAcquisitionPending)
DECLARE_RTI_EXCEPTION(FederateOwnsAttributes)

// Time Management
DECLARE_RTI_EXCEPTION(InvalidLogicalTime)
DECLARE_RTI_EXCEPTION(InvalidLookahead)
DECLARE_RTI_EXCEPTION(TimeRegulationAlreadyEnabled)
DECLARE_RTI_EXCEPTION(TimeConstrainedAlreadyEnabled)
DECLARE_RTI_EXCEPTION(InTimeAdvancingState)
DECLARE_RTI_EXCEPTION(RequestForTimeRegulationPending)
DECLARE_RTI_EXCEPTION(RequestForTimeConstrainedPending)
DECLARE_RTI_EXCEPTION(LogicalTimeAlreadyPassed)

// DDM
DECLARE_RTI_EXCEPTION(InvalidRegion)
DECLARE_RTI_EXCEPTION(InvalidRangeBound)
DECLARE_RTI_EXCEPTION(InvalidDimensionHandle)

// Synchronization, Save & Restore
DECLARE_RTI_EXCEPTION(SaveInProgress)
DECLARE_RTI_EXCEPTION(RestoreInProgress)
DECLARE_RTI_EXCEPTION(InvalidResignAction)

#undef DECLARE_RTI_EXCEPTION

} // namespace rti1516e

#endif // RTI1516E_RTIEXCEPTION_H
