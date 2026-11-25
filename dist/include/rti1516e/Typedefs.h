// Typedefs.h - Official IEEE 1516e Handles and Data Types
#ifndef RTI1516E_TYPEDEFS_H
#define RTI1516E_TYPEDEFS_H

#include <cstdint>
#include <string>
#include <vector>
#include <set>
#include <map>
#include <cstring>

namespace rti1516e {

class ObjectClassHandle {
public:
    ObjectClassHandle() : _val(0) {}
    explicit ObjectClassHandle(int32_t val) : _val(val) {}
    int32_t getValue() const { return _val; }
    bool isValid() const { return _val > 0; }
    bool operator==(const ObjectClassHandle& o) const { return _val == o._val; }
    bool operator!=(const ObjectClassHandle& o) const { return _val != o._val; }
    bool operator<(const ObjectClassHandle& o) const { return _val < o._val; }
private:
    int32_t _val;
};

class AttributeHandle {
public:
    AttributeHandle() : _val(0) {}
    explicit AttributeHandle(int32_t val) : _val(val) {}
    int32_t getValue() const { return _val; }
    bool isValid() const { return _val > 0; }
    bool operator==(const AttributeHandle& o) const { return _val == o._val; }
    bool operator!=(const AttributeHandle& o) const { return _val != o._val; }
    bool operator<(const AttributeHandle& o) const { return _val < o._val; }
private:
    int32_t _val;
};

class InteractionClassHandle {
public:
    InteractionClassHandle() : _val(0) {}
    explicit InteractionClassHandle(int32_t val) : _val(val) {}
    int32_t getValue() const { return _val; }
    bool isValid() const { return _val > 0; }
    bool operator==(const InteractionClassHandle& o) const { return _val == o._val; }
    bool operator!=(const InteractionClassHandle& o) const { return _val != o._val; }
    bool operator<(const InteractionClassHandle& o) const { return _val < o._val; }
private:
    int32_t _val;
};

class ParameterHandle {
public:
    ParameterHandle() : _val(0) {}
    explicit ParameterHandle(int32_t val) : _val(val) {}
    int32_t getValue() const { return _val; }
    bool isValid() const { return _val > 0; }
    bool operator==(const ParameterHandle& o) const { return _val == o._val; }
    bool operator!=(const ParameterHandle& o) const { return _val != o._val; }
    bool operator<(const ParameterHandle& o) const { return _val < o._val; }
private:
    int32_t _val;
};

class ObjectInstanceHandle {
public:
    ObjectInstanceHandle() : _val(0) {}
    explicit ObjectInstanceHandle(int32_t val) : _val(val) {}
    int32_t getValue() const { return _val; }
    bool isValid() const { return _val > 0; }
    bool operator==(const ObjectInstanceHandle& o) const { return _val == o._val; }
    bool operator!=(const ObjectInstanceHandle& o) const { return _val != o._val; }
    bool operator<(const ObjectInstanceHandle& o) const { return _val < o._val; }
private:
    int32_t _val;
};

class FederateHandle {
public:
    FederateHandle() : _val(0) {}
    explicit FederateHandle(int32_t val) : _val(val) {}
    int32_t getValue() const { return _val; }
    bool isValid() const { return _val > 0; }
    bool operator==(const FederateHandle& o) const { return _val == o._val; }
    bool operator!=(const FederateHandle& o) const { return _val != o._val; }
    bool operator<(const FederateHandle& o) const { return _val < o._val; }
private:
    int32_t _val;
};

class DimensionHandle {
public:
    DimensionHandle() : _val(0) {}
    explicit DimensionHandle(int32_t val) : _val(val) {}
    int32_t getValue() const { return _val; }
    bool isValid() const { return _val > 0; }
    bool operator==(const DimensionHandle& o) const { return _val == o._val; }
    bool operator!=(const DimensionHandle& o) const { return _val != o._val; }
    bool operator<(const DimensionHandle& o) const { return _val < o._val; }
private:
    int32_t _val;
};

class RegionHandle {
public:
    RegionHandle() : _val(0) {}
    explicit RegionHandle(uint32_t val) : _val(val) {}
    uint32_t getValue() const { return _val; }
    bool isValid() const { return _val > 0; }
    bool operator==(const RegionHandle& o) const { return _val == o._val; }
    bool operator!=(const RegionHandle& o) const { return _val != o._val; }
    bool operator<(const RegionHandle& o) const { return _val < o._val; }
private:
    uint32_t _val;
};

struct RangeBounds {
    uint32_t lowerBound;
    uint32_t upperBound;
    RangeBounds(uint32_t l, uint32_t u) : lowerBound(l), upperBound(u) {}
    RangeBounds() : lowerBound(0), upperBound(0) {}
};

class VariableLengthData {
public:
    VariableLengthData() {}
    VariableLengthData(const void* inData, size_t inSize) {
        setData(inData, inSize);
    }
    VariableLengthData(const VariableLengthData& rhs) : _data(rhs._data) {}
    VariableLengthData& operator=(const VariableLengthData& rhs) {
        if (this != &rhs) _data = rhs._data;
        return *this;
    }
    void setData(const void* inData, size_t inSize) {
        if (inData && inSize > 0) {
            const uint8_t* p = static_cast<const uint8_t*>(inData);
            _data.assign(p, p + inSize);
        } else {
            _data.clear();
        }
    }
    const void* data() const { return _data.empty() ? nullptr : _data.data(); }
    size_t size() const { return _data.size(); }
    const uint8_t* getBuffer() const { return _data.data(); }
private:
    std::vector<uint8_t> _data;
};

using AttributeHandleSet = std::set<AttributeHandle>;
using AttributeHandleValueMap = std::map<AttributeHandle, VariableLengthData>;
using ParameterHandleValueMap = std::map<ParameterHandle, VariableLengthData>;
using DimensionHandleSet = std::set<DimensionHandle>;

} // namespace rti1516e

#endif // RTI1516E_TYPEDEFS_H
