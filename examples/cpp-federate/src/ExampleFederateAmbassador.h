#ifndef EXAMPLE_FEDERATE_AMBASSADOR_H
#define EXAMPLE_FEDERATE_AMBASSADOR_H

#include <rti1516e/NullFederateAmbassador.h>
#include <iostream>
#include <atomic>

class ExampleFederateAmbassador : public rti1516e::NullFederateAmbassador {
public:
    ExampleFederateAmbassador()
        : m_discoverCount(0), m_reflectCount(0), m_removeCount(0) {}

    virtual ~ExampleFederateAmbassador() throw() {}

    virtual void discoverObjectInstance(
        rti1516e::ObjectInstanceHandle theObject,
        rti1516e::ObjectClassHandle theObjectClass,
        const std::wstring& theObjectName) throw(rti1516e::FederateInternalError) override {
        m_discoverCount++;
        std::wcout << L"  [Ambassador Callback] >>> OBJECT DISCOVERED: name='"
                   << theObjectName << L"', handle=" << theObject.getValue()
                   << L", class=" << theObjectClass.getValue() << std::endl;
    }

    virtual void reflectAttributeValues(
        rti1516e::ObjectInstanceHandle theObject,
        const rti1516e::AttributeHandleValueMap& theAttributeValues,
        const rti1516e::VariableLengthData& userSuppliedTag,
        rti1516e::OrderType sentOrder,
        rti1516e::TransportationTypeHandle theType) throw(rti1516e::FederateInternalError) override {
        m_reflectCount++;
        std::cout << "  [Ambassador Callback] >>> ATTRIBUTES REFLECTED for object "
                  << theObject.getValue() << " (" << theAttributeValues.size() << " attributes):" << std::endl;
        for (const auto& entry : theAttributeValues) {
            std::string valStr(reinterpret_cast<const char*>(entry.second.data()), entry.second.size());
            std::cout << "      * Attr " << entry.first.getValue() << " => \"" << valStr << "\"" << std::endl;
        }
    }

    virtual void removeObjectInstance(
        rti1516e::ObjectInstanceHandle theObject,
        const rti1516e::VariableLengthData& userSuppliedTag,
        rti1516e::OrderType sentOrder) throw(rti1516e::FederateInternalError) override {
        m_removeCount++;
        std::cout << "  [Ambassador Callback] >>> OBJECT REMOVED: handle="
                  << theObject.getValue() << std::endl;
    }

    virtual void timeAdvanceGrant(double theTime) throw(rti1516e::FederateInternalError) override {
        std::cout << "  [Ambassador Callback] >>> TIME ADVANCE GRANTED: time="
                  << theTime << std::endl;
    }

    int getDiscoverCount() const { return m_discoverCount.load(); }
    int getReflectCount() const { return m_reflectCount.load(); }
    int getRemoveCount() const { return m_removeCount.load(); }

private:
    std::atomic<int> m_discoverCount;
    std::atomic<int> m_reflectCount;
    std::atomic<int> m_removeCount;
};

#endif // EXAMPLE_FEDERATE_AMBASSADOR_H
