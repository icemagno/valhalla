#ifndef RTI1516E_ENUMS_H
#define RTI1516E_ENUMS_H

namespace rti1516e {

enum class CallbackModel {
    HLA_IMMEDIATE,
    HLA_EVOKED
};

enum class OrderType {
    RECEIVE,
    TIMESTAMP
};

enum class ResignAction {
    UNCONDITIONALLY_DIVEST_ATTRIBUTES,
    DELETE_OBJECTS,
    CANCEL_PENDING_OWNERSHIP_ACQUISITIONS,
    DELETE_OBJECTS_THEN_DIVEST,
    CANCEL_THEN_DELETE_THEN_DIVEST,
    NO_ACTION
};

enum class TransportationTypeHandle {
    HLA_RELIABLE,
    HLA_BEST_EFFORT
};

enum class SynchronizationPointFailureReason {
    SYNCHRONIZATION_POINT_LABEL_NOT_UNIQUE,
    SYNCHRONIZATION_SET_MEMBER_NOT_JOINED
};

} // namespace rti1516e

#endif // RTI1516E_ENUMS_H
