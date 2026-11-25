/*
 * Project Valhalla - Advanced Telemetry, DDM, Ownership, Save/Restore & NMR Federate
 * Compliant with IEEE 1516-2010 (HLA Evolved)
 */
package ieee1516e;

import hla.rti1516e.AttributeHandle;
import hla.rti1516e.AttributeHandleSet;
import hla.rti1516e.AttributeHandleValueMap;
import hla.rti1516e.CallbackModel;
import hla.rti1516e.DimensionHandle;
import hla.rti1516e.DimensionHandleSet;
import hla.rti1516e.InteractionClassHandle;
import hla.rti1516e.ObjectClassHandle;
import hla.rti1516e.ObjectInstanceHandle;
import hla.rti1516e.ParameterHandle;
import hla.rti1516e.ParameterHandleValueMap;
import hla.rti1516e.RangeBounds;
import hla.rti1516e.RegionHandle;
import hla.rti1516e.RegionHandleSet;
import hla.rti1516e.ResignAction;
import hla.rti1516e.RtiFactoryFactory;
import hla.rti1516e.RTIambassador;
import hla.rti1516e.encoding.EncoderFactory;
import hla.rti1516e.encoding.HLAASCIIstring;
import hla.rti1516e.encoding.HLAinteger16BE;
import hla.rti1516e.encoding.HLAinteger32BE;
import hla.rti1516e.exceptions.FederatesCurrentlyJoined;
import hla.rti1516e.exceptions.FederationExecutionAlreadyExists;
import hla.rti1516e.exceptions.FederationExecutionDoesNotExist;
import hla.rti1516e.exceptions.RTIexception;
import hla.rti1516e.time.HLAfloat64Interval;
import hla.rti1516e.time.HLAfloat64Time;
import hla.rti1516e.time.HLAfloat64TimeFactory;

import java.io.BufferedReader;
import java.io.File;
import java.io.InputStreamReader;
import java.net.URL;

public class ExampleFederate
{
	public static final int ITERATIONS = 30;
	public static final String READY_TO_RUN = "ReadyToRun";

	private RTIambassador rtiamb;
	private ExampleFederateAmbassador fedamb;
	private HLAfloat64TimeFactory timeFactory;
	protected EncoderFactory encoderFactory;

	// Metamodel Caches
	protected ObjectClassHandle entityClassHandle;
	protected AttributeHandle positionHandle;
	protected AttributeHandle velocityHandle;
	protected AttributeHandle payloadHandle;
	protected AttributeHandle batteryHandle;

	protected InteractionClassHandle sensorTrackHandle;
	protected ParameterHandle trackIdHandle;
	protected ParameterHandle sensorDataHandle;

	protected InteractionClassHandle missionControlHandle;
	protected ParameterHandle commandCodeHandle;
	protected ParameterHandle targetLocationHandle;

	// DDM Caches
	protected DimensionHandle dimXHandle;
	protected DimensionHandle dimYHandle;
	protected DimensionHandle dimZHandle;

	public RTIambassador getRtiAmbassador()
	{
		return this.rtiamb;
	}

	private void log( String message )
	{
		System.out.println( "AdvancedFederate  : " + message );
	}

	private void waitForUser()
	{
		String auto = System.getProperty("auto.run");
		if( "true".equalsIgnoreCase(auto) )
		{
			log( " [auto.run=true] Bypassing manual pause..." );
			return;
		}
		log( " >>>>>>>>>> Press Enter to Continue <<<<<<<<<<" );
		BufferedReader reader = new BufferedReader( new InputStreamReader(System.in) );
		try
		{
			reader.readLine();
		}
		catch( Exception e )
		{
			log( "Error while waiting for user input: " + e.getMessage() );
		}
	}

	public void runFederate( String federateName ) throws Exception
	{
		/////////////////////////////////////////////////
		// 1 & 2. Create RTIambassador and Connect     //
		/////////////////////////////////////////////////
		log( "Creating RTIambassador and EncoderFactory..." );
		rtiamb = RtiFactoryFactory.getRtiFactory().getRtiAmbassador();
		encoderFactory = RtiFactoryFactory.getRtiFactory().getEncoderFactory();

		log( "Connecting to Valhalla Central RTI Component..." );
		fedamb = new ExampleFederateAmbassador( this );
		rtiamb.connect( fedamb, CallbackModel.HLA_IMMEDIATE );
		log( "Connected successfully via Portico TCP transport." );

		/////////////////////////////////////////////////
		// 3. Create Federation with ValhallaAdvanced   //
		/////////////////////////////////////////////////
		log( "Creating Federation Execution 'ValhallaAdvancedFed'..." );
		try
		{
			URL[] modules = new URL[]{
				(new File("foms/ValhallaAdvanced.xml")).toURI().toURL()
			};
			rtiamb.createFederationExecution( "ValhallaAdvancedFed", modules );
			log( "SUCCESS Created federation execution [ValhallaAdvancedFed]" );
		}
		catch( FederationExecutionAlreadyExists exists )
		{
			log( "Federation execution already exists, proceeding to join." );
		}

		/////////////////////////////////////////////////
		// 4. Join Federation Execution               //
		/////////////////////////////////////////////////
		log( "Joining federation execution as [" + federateName + "]..." );
		rtiamb.joinFederationExecution( federateName, "AdvancedFederateType", "ValhallaAdvancedFed", new URL[0] );
		log( "SUCCESS Joined to federation [ValhallaAdvancedFed]" );
		this.timeFactory = (HLAfloat64TimeFactory)rtiamb.getTimeFactory();

		/////////////////////////////////////////////////
		// 5. Synchronization Point Barrier            //
		/////////////////////////////////////////////////
		log( "Registering synchronization point: " + READY_TO_RUN );
		rtiamb.registerFederationSynchronizationPoint( READY_TO_RUN, null );
		while( !fedamb.isAnnounced )
		{
			rtiamb.evokeMultipleCallbacks( 0.05, 0.1 );
		}

		waitForUser();

		rtiamb.synchronizationPointAchieved( READY_TO_RUN );
		log( "Achieved sync point: " + READY_TO_RUN + ", waiting for federation..." );
		while( !fedamb.isReadyToRun )
		{
			rtiamb.evokeMultipleCallbacks( 0.05, 0.1 );
		}
		log( "Federation Synchronized successfully!" );

		/////////////////////////////////////////////////
		// 6. Enable Time Policies                     //
		/////////////////////////////////////////////////
		enableTimePolicy();
		log( "Time Policy Enabled (Time Regulated + Time Constrained)." );

		/////////////////////////////////////////////////
		// 7. Publish and Subscribe                    //
		/////////////////////////////////////////////////
		publishAndSubscribe();
		log( "Published and Subscribed to Advanced Classes." );

		/////////////////////////////////////////////////
		// 8. Register Object Instance                 //
		/////////////////////////////////////////////////
		ObjectInstanceHandle objectHandle = rtiamb.registerObjectInstance( entityClassHandle, "TacticalAsset_01" );
		log( "Registered TacticalEntity, handle=" + objectHandle );

		/////////////////////////////////////////////////
		// 9. PHASE 1: Event-Driven Time (NMR / FQR)   //
		/////////////////////////////////////////////////
		log( "\n=== PHASE 1: Event-Driven Time Advance (NextMessageRequest) ===" );
		for( int i = 1; i <= 5; i++ )
		{
			updateAttributeValues( objectHandle, i );
			sendSensorInteraction( i );
			advanceTimeNMR( 1.0 );
			log( "[NMR Step " + i + "] Time Advanced to " + fedamb.federateTime );
		}

		/////////////////////////////////////////////////
		// 10. PHASE 2: Ownership Management           //
		/////////////////////////////////////////////////
		log( "\n=== PHASE 2: Ownership Management (Divestiture & Query) ===" );
		AttributeHandleSet divestSet = rtiamb.getAttributeHandleSetFactory().create();
		divestSet.add( batteryHandle );
		try
		{
			log( "Divesting ownership of attribute BatteryLevel..." );
			rtiamb.unconditionalAttributeOwnershipDivestiture( objectHandle, divestSet );
			log( "Ownership divestiture sent." );

			log( "Querying ownership of attribute BatteryLevel..." );
			rtiamb.queryAttributeOwnership( objectHandle, batteryHandle );
		}
		catch( Exception e )
		{
			log( "Ownership management note: " + e.getMessage() );
		}

		/////////////////////////////////////////////////
		// 11. PHASE 3: Federation Save & Restore      //
		/////////////////////////////////////////////////
		log( "\n=== PHASE 3: Federation Save & Restore Checkpointing ===" );
		try
		{
			String saveLabel = "Checkpoint_Valhalla";
			log( "Requesting Federation Save: " + saveLabel );
			rtiamb.requestFederationSave( saveLabel );
			int waitCount = 0;
			while( !fedamb.isSaved && waitCount < 30 )
			{
				rtiamb.evokeMultipleCallbacks( 0.05, 0.1 );
				waitCount++;
			}

			if( fedamb.isSaved )
			{
				log( "Requesting Federation Restore: " + saveLabel );
				rtiamb.requestFederationRestore( saveLabel );
				waitCount = 0;
				while( !fedamb.isRestored && waitCount < 30 )
				{
					rtiamb.evokeMultipleCallbacks( 0.05, 0.1 );
					waitCount++;
				}
			}
		}
		catch( Exception e )
		{
			log( "Save/Restore note: " + e.getMessage() );
		}

		/////////////////////////////////////////////////
		// 12. PHASE 4: Simulation Burst & Throughput  //
		/////////////////////////////////////////////////
		log( "\n=== PHASE 4: High-Throughput Simulation Loop (" + ITERATIONS + " iterations) ===" );
		long startNano = System.nanoTime();
		for( int i = 6; i <= ITERATIONS; i++ )
		{
			updateAttributeValues( objectHandle, i );
			sendSensorInteraction( i );
			advanceTimeTAR( 1.0 );
		}
		long elapsedMs = (System.nanoTime() - startNano) / 1000000;
		log( "Simulation burst completed! Total time: " + elapsedMs + " ms for " + (ITERATIONS - 5) + " cycles." );

		/////////////////////////////////////////////////
		// 13. Teardown & Clean Disconnect             //
		/////////////////////////////////////////////////
		log( "\nDeleting registered object instance..." );
		rtiamb.deleteObjectInstance( objectHandle, generateTag() );
		log( "Deleted Object, handle=" + objectHandle );

		log( "Resigning from federation execution..." );
		rtiamb.resignFederationExecution( ResignAction.DELETE_OBJECTS );
		log( "Resigned from Federation successfully." );

		try
		{
			rtiamb.destroyFederationExecution( "ValhallaAdvancedFed" );
			log( "Destroyed Federation [ValhallaAdvancedFed]" );
		}
		catch( Exception e )
		{
			log( "Federation teardown: " + e.getMessage() );
		}

		rtiamb.disconnect();
		log( "Disconnected gracefully from Valhalla CRC." );
	}

	private void enableTimePolicy() throws Exception
	{
		HLAfloat64Interval lookahead = timeFactory.makeInterval( fedamb.federateLookahead );
		rtiamb.enableTimeRegulation( lookahead );
		while( !fedamb.isRegulating )
		{
			rtiamb.evokeMultipleCallbacks( 0.05, 0.1 );
		}

		rtiamb.enableTimeConstrained();
		while( !fedamb.isConstrained )
		{
			rtiamb.evokeMultipleCallbacks( 0.05, 0.1 );
		}
	}

	private void publishAndSubscribe() throws RTIexception
	{
		// 1. Resolve Object Class & Attributes
		this.entityClassHandle = rtiamb.getObjectClassHandle( "HLAobjectRoot.TacticalEntity" );
		this.positionHandle = rtiamb.getAttributeHandle( entityClassHandle, "Position" );
		this.velocityHandle = rtiamb.getAttributeHandle( entityClassHandle, "Velocity" );
		this.payloadHandle = rtiamb.getAttributeHandle( entityClassHandle, "PayloadStatus" );
		this.batteryHandle = rtiamb.getAttributeHandle( entityClassHandle, "BatteryLevel" );

		AttributeHandleSet attrSet = rtiamb.getAttributeHandleSetFactory().create();
		attrSet.add( positionHandle );
		attrSet.add( velocityHandle );
		attrSet.add( payloadHandle );
		attrSet.add( batteryHandle );

		rtiamb.publishObjectClassAttributes( entityClassHandle, attrSet );
		rtiamb.subscribeObjectClassAttributes( entityClassHandle, attrSet );

		// 2. Resolve Interaction Classes & Parameters
		this.sensorTrackHandle = rtiamb.getInteractionClassHandle( "HLAinteractionRoot.SensorTrack" );
		this.trackIdHandle = rtiamb.getParameterHandle( sensorTrackHandle, "TrackID" );
		this.sensorDataHandle = rtiamb.getParameterHandle( sensorTrackHandle, "SensorData" );

		rtiamb.publishInteractionClass( sensorTrackHandle );
		rtiamb.subscribeInteractionClass( sensorTrackHandle );

		this.missionControlHandle = rtiamb.getInteractionClassHandle( "HLAinteractionRoot.MissionControl" );
		this.commandCodeHandle = rtiamb.getParameterHandle( missionControlHandle, "CommandCode" );
		this.targetLocationHandle = rtiamb.getParameterHandle( missionControlHandle, "TargetLocation" );

		rtiamb.publishInteractionClass( missionControlHandle );
		rtiamb.subscribeInteractionClass( missionControlHandle );
	}

	private void updateAttributeValues( ObjectInstanceHandle objectHandle, int step ) throws RTIexception
	{
		AttributeHandleValueMap attributes = rtiamb.getAttributeHandleValueMapFactory().create(4);

		// Position string
		String posStr = String.format( "Lat:%.4f,Lon:%.4f,Alt:%.1f", -23.55 + step * 0.01, -46.63 + step * 0.01, 1000.0 + step * 50.0 );
		attributes.put( positionHandle, posStr.getBytes() );

		// Velocity int32
		HLAinteger32BE vel = encoderFactory.createHLAinteger32BE( 250 + step * 5 );
		attributes.put( velocityHandle, vel.toByteArray() );

		// PayloadStatus string
		String payload = (step % 2 == 0) ? "RADAR_ACTIVE_SCANNING" : "STANDBY_PASSIVE_ACOUSTIC";
		attributes.put( payloadHandle, payload.getBytes() );

		// BatteryLevel int16
		HLAinteger16BE battery = encoderFactory.createHLAinteger16BE( (short)(100 - step) );
		attributes.put( batteryHandle, battery.toByteArray() );

		// Send RO update
		rtiamb.updateAttributeValues( objectHandle, attributes, generateTag() );

		// Send TSO update
		HLAfloat64Time time = timeFactory.makeTime( fedamb.federateTime + fedamb.federateLookahead );
		rtiamb.updateAttributeValues( objectHandle, attributes, generateTag(), time );
	}

	private void sendSensorInteraction( int step ) throws RTIexception
	{
		ParameterHandleValueMap parameters = rtiamb.getParameterHandleValueMapFactory().create(2);

		HLAinteger32BE trackId = encoderFactory.createHLAinteger32BE( 1000 + step );
		parameters.put( trackIdHandle, trackId.toByteArray() );

		String data = "TargetTrack_Bearing=" + (step * 12 % 360) + "_Range=" + (5000 - step * 100);
		parameters.put( sensorDataHandle, data.getBytes() );

		// Send RO interaction
		rtiamb.sendInteraction( sensorTrackHandle, parameters, generateTag() );

		// Send TSO interaction
		HLAfloat64Time time = timeFactory.makeTime( fedamb.federateTime + fedamb.federateLookahead );
		rtiamb.sendInteraction( sensorTrackHandle, parameters, generateTag(), time );
	}

	private void advanceTimeNMR( double timestep ) throws RTIexception
	{
		fedamb.isAdvancing = true;
		HLAfloat64Time newTime = timeFactory.makeTime( fedamb.federateTime + timestep );
		rtiamb.nextMessageRequest( newTime );
		while( fedamb.isAdvancing )
		{
			rtiamb.evokeMultipleCallbacks( 0.05, 0.1 );
		}
	}

	private void advanceTimeTAR( double timestep ) throws RTIexception
	{
		fedamb.isAdvancing = true;
		HLAfloat64Time newTime = timeFactory.makeTime( fedamb.federateTime + timestep );
		rtiamb.timeAdvanceRequest( newTime );
		while( fedamb.isAdvancing )
		{
			rtiamb.evokeMultipleCallbacks( 0.05, 0.1 );
		}
	}

	private byte[] generateTag()
	{
		return ("Tag_" + System.currentTimeMillis()).getBytes();
	}

	public static void main( String[] args )
	{
		String federateName = "AdvancedFederate";
		if( args.length > 0 )
		{
			federateName = args[0];
		}

		try
		{
			new ExampleFederate().runFederate( federateName );
		}
		catch( Exception e )
		{
			e.printStackTrace();
		}
	}
}
