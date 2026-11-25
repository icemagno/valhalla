/*
 * Project Valhalla - Advanced Telemetry & Stress Federate Ambassador
 * Compliant with IEEE 1516-2010 (HLA Evolved)
 */
package ieee1516e;

import hla.rti1516e.AttributeHandle;
import hla.rti1516e.AttributeHandleSet;
import hla.rti1516e.AttributeHandleValueMap;
import hla.rti1516e.FederateHandle;
import hla.rti1516e.FederateHandleSet;
import hla.rti1516e.InteractionClassHandle;
import hla.rti1516e.LogicalTime;
import hla.rti1516e.NullFederateAmbassador;
import hla.rti1516e.ObjectClassHandle;
import hla.rti1516e.ObjectInstanceHandle;
import hla.rti1516e.OrderType;
import hla.rti1516e.ParameterHandle;
import hla.rti1516e.ParameterHandleValueMap;
import hla.rti1516e.RestoreFailureReason;
import hla.rti1516e.SaveFailureReason;
import hla.rti1516e.SynchronizationPointFailureReason;
import hla.rti1516e.TransportationTypeHandle;
import hla.rti1516e.encoding.DecoderException;
import hla.rti1516e.encoding.HLAASCIIstring;
import hla.rti1516e.encoding.HLAinteger16BE;
import hla.rti1516e.encoding.HLAinteger32BE;
import hla.rti1516e.exceptions.FederateInternalError;
import hla.rti1516e.time.HLAfloat64Time;

public class ExampleFederateAmbassador extends NullFederateAmbassador
{
	private ExampleFederate federate;

	protected double federateTime        = 0.0;
	protected double federateLookahead   = 1.0;

	protected boolean isRegulating       = false;
	protected boolean isConstrained      = false;
	protected boolean isAdvancing        = false;

	protected boolean isAnnounced        = false;
	protected boolean isReadyToRun       = false;

	// Save & Restore tracking
	protected boolean isSaveInitiated    = false;
	protected boolean isSaved            = false;
	protected boolean isRestoreInitiated = false;
	protected boolean isRestored         = false;

	public ExampleFederateAmbassador( ExampleFederate federate )
	{
		this.federate = federate;
	}

	private void log( String message )
	{
		System.out.println( "FederateAmbassador: " + message );
	}

	@Override
	public void synchronizationPointRegistrationFailed( String label,
	                                                    SynchronizationPointFailureReason reason )
	{
		log( "Failed to register sync point: " + label + ", reason=" + reason );
	}

	@Override
	public void synchronizationPointRegistrationSucceeded( String label )
	{
		log( "Successfully registered sync point: " + label );
	}

	@Override
	public void announceSynchronizationPoint( String label, byte[] tag )
	{
		log( "Synchronization point announced: " + label );
		if( label.equals(ExampleFederate.READY_TO_RUN) )
			this.isAnnounced = true;
	}

	@Override
	public void federationSynchronized( String label, FederateHandleSet failedSet )
	{
		log( "Federation Synchronized: " + label );
		if( label.equals(ExampleFederate.READY_TO_RUN) )
			this.isReadyToRun = true;
	}

	@Override
	public void timeRegulationEnabled( LogicalTime time )
	{
		this.federateTime = ((HLAfloat64Time)time).getValue();
		this.isRegulating = true;
	}

	@Override
	public void timeConstrainedEnabled( LogicalTime time )
	{
		this.federateTime = ((HLAfloat64Time)time).getValue();
		this.isConstrained = true;
	}

	@Override
	public void timeAdvanceGrant( LogicalTime time )
	{
		this.federateTime = ((HLAfloat64Time)time).getValue();
		this.isAdvancing = false;
	}

	@Override
	public void discoverObjectInstance( ObjectInstanceHandle theObject,
	                                    ObjectClassHandle theObjectClass,
	                                    String objectName )
	{
		log( "Discover Object: handle=" + theObject + ", class=" + theObjectClass + ", name=" + objectName );
	}

	@Override
	public void reflectAttributeValues( ObjectInstanceHandle theObject,
	                                    AttributeHandleValueMap theAttributes,
	                                    byte[] tag,
	                                    OrderType sentOrder,
	                                    TransportationTypeHandle transport,
	                                    SupplementalReflectInfo reflectInfo )
	    throws FederateInternalError
	{
		StringBuilder builder = new StringBuilder( "Reflection for Object [" + theObject + "]: " );
		for( AttributeHandle handle : theAttributes.keySet() )
		{
			byte[] val = theAttributes.get( handle );
			builder.append( "attr[" + handle + "]=" + new String(val) + " " );
		}
		log( builder.toString() );
	}

	@Override
	public void reflectAttributeValues( ObjectInstanceHandle theObject,
	                                    AttributeHandleValueMap theAttributes,
	                                    byte[] tag,
	                                    OrderType sentOrder,
	                                    TransportationTypeHandle transport,
	                                    LogicalTime time,
	                                    OrderType receivedOrder,
	                                    SupplementalReflectInfo reflectInfo )
	    throws FederateInternalError
	{
		StringBuilder builder = new StringBuilder( "Reflection (TSO @" + ((HLAfloat64Time)time).getValue() + ") for Object [" + theObject + "]: " );
		for( AttributeHandle handle : theAttributes.keySet() )
		{
			byte[] val = theAttributes.get( handle );
			builder.append( "attr[" + handle + "]=" + new String(val) + " " );
		}
		log( builder.toString() );
	}

	@Override
	public void receiveInteraction( InteractionClassHandle interactionClass,
	                                ParameterHandleValueMap theParameters,
	                                byte[] tag,
	                                OrderType sentOrdering,
	                                TransportationTypeHandle theTransport,
	                                SupplementalReceiveInfo receiveInfo )
	{
		log( "Receive Interaction [" + interactionClass + "] with " + theParameters.size() + " parameters" );
	}

	@Override
	public void receiveInteraction( InteractionClassHandle interactionClass,
	                                ParameterHandleValueMap theParameters,
	                                byte[] tag,
	                                OrderType sentOrdering,
	                                TransportationTypeHandle theTransport,
	                                LogicalTime time,
	                                OrderType receivedOrdering,
	                                SupplementalReceiveInfo receiveInfo )
	{
		log( "Receive Interaction (TSO @" + ((HLAfloat64Time)time).getValue() + ") [" + interactionClass + "] with " + theParameters.size() + " parameters" );
	}

	@Override
	public void removeObjectInstance( ObjectInstanceHandle theObject,
	                                  byte[] userSuppliedTag,
	                                  OrderType sentOrdering,
	                                  SupplementalRemoveInfo removeInfo )
	{
		log( "Removed Object: handle=" + theObject );
	}

	// ==========================================================
	//  Save & Restore Callbacks
	// ==========================================================
	@Override
	public void initiateFederateSave( String label ) throws FederateInternalError
	{
		log( "Initiate Federate Save requested: label=" + label );
		this.isSaveInitiated = true;
		try
		{
			federate.getRtiAmbassador().federateSaveBegun();
			federate.getRtiAmbassador().federateSaveComplete();
			log( "Federate save completed locally for label: " + label );
		}
		catch( Exception e )
		{
			log( "Error completing federate save: " + e.getMessage() );
		}
	}

	@Override
	public void federationSaved()
	{
		log( ">>> FEDERATION FULLY SAVED!" );
		this.isSaved = true;
	}

	@Override
	public void federationNotSaved( SaveFailureReason reason )
	{
		log( "[-] Federation Save FAILED: reason=" + reason );
	}

	@Override
	public void initiateFederateRestore( String label, String federateName, FederateHandle handle )
	{
		log( "Initiate Federate Restore requested: label=" + label + ", fed=" + federateName );
		this.isRestoreInitiated = true;
		try
		{
			federate.getRtiAmbassador().federateRestoreComplete();
			log( "Federate restore completed locally for label: " + label );
		}
		catch( Exception e )
		{
			log( "Error completing federate restore: " + e.getMessage() );
		}
	}

	@Override
	public void federationRestored()
	{
		log( ">>> FEDERATION FULLY RESTORED!" );
		this.isRestored = true;
	}

	@Override
	public void federationNotRestored( RestoreFailureReason reason )
	{
		log( "[-] Federation Restore FAILED: reason=" + reason );
	}

	// ==========================================================
	//  Ownership Management Callbacks
	// ==========================================================
	@Override
	public void requestAttributeOwnershipAssumption( ObjectInstanceHandle theObject,
	                                                AttributeHandleSet offeredAttributes,
	                                                byte[] userSuppliedTag )
	{
		log( "Attribute ownership assumption requested for object: " + theObject + ", attrs=" + offeredAttributes.size() );
	}

	@Override
	public void attributeOwnershipAcquisitionNotification( ObjectInstanceHandle theObject,
	                                                      AttributeHandleSet securedAttributes,
	                                                      byte[] userSuppliedTag )
	{
		log( "Attribute ownership ACQUIRED for object: " + theObject + ", attrs=" + securedAttributes.size() );
	}

	@Override
	public void informAttributeOwnership( ObjectInstanceHandle theObject,
	                                      AttributeHandle theAttribute,
	                                      FederateHandle theOwner )
	{
		log( "Attribute ownership query response: object=" + theObject + ", attr=" + theAttribute + ", owner=" + theOwner );
	}
}
