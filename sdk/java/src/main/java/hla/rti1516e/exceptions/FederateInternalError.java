package hla.rti1516e.exceptions;

public class FederateInternalError extends RTIexception {
    public FederateInternalError(String message) { super(message); }
    public FederateInternalError(String message, Throwable cause) { super(message, cause); }
}
