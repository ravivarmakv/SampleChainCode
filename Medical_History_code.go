package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"encoding/json"
	"regexp"
)

var logger = shim.NewLogger("CLDChaincode")



//==============================================================================================================================
//	 Participant types - Each participant type is mapped to an integer which we use to compare to the value stored in a
//						 user's eCert
//==============================================================================================================================
//CURRENT WORKAROUND USES ROLES CHANGE WHEN OWN USERS CAN BE CREATED SO THAT IT READ 1, 2, 3, 4
const   PARENTS   	=  "parents"
const   BIRTHDAY      	=  "birthday"
const   HEALTHY  	=  "healthy"
const   ILLNESS 	=  "illness"
const	DEATH		=  "death"

//==============================================================================================================================
//	 Status types - Asset lifecycle is broken down into 5 statuses, this is part of the business logic to determine what can
//					be done to the member at points in it's lifecycle
//==============================================================================================================================
const   STATE_CARRYING 			=  0
const   STATE_BIRTH	  		=  1
const   STATE_HEALTHY		 	=  2
const   STATE_ILLNESS	 		=  3
const 	STATE_DEATH			=  4

type  SimpleChaincode struct {
}


//==============================================================================================================================
//	member - Defines the structure for a car object. JSON on right tells it what JSON fields to map to
//			  that element when reading a JSON object into the struct e.g. JSON make -> Struct Make.
//==============================================================================================================================
type Member struct {
	Name   	        string `json:"name"`
	DOB    	        string `json:"DOB"`
	Gender          string `json:"gender"`
	BloodGrp       string `json:"BloodGrp"`
	Weight          int    `json:"Weight"`
	Status          int    `json:"status"`
	Dead		bool   `json:"dead"`
	ILNSID 		string `json:"IllnessID"`
}


//==============================================================================================================================
//	Ilns Holder - Defines the structure that holds all the Illness for Entity that have been created.
//				Used as an index when querying all reports.
//==============================================================================================================================

type ILNS_Holder struct {
	ILNSs 	[]string `json:"ILNSs"`
}



//==============================================================================================================================
//	User_and_eCert - Struct for storing the JSON of a user and their ecert
//==============================================================================================================================

type User_and_eCert struct {
	Identity string `json:"identity"`
	eCert string `json:"ecert"`
}



//==============================================================================================================================
//	Init Function - Called when the user deploys the chaincode
//==============================================================================================================================
func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	//Args
	//				0
	//			peer_address

	var ILNSIDs ILNS_Holder

	bytes, err := json.Marshal(ILNSIDs)

    if err != nil { return nil, errors.New("Error creating ILNS_Holder record") }

	err = stub.PutState("ILNSIDs", bytes)

	for i:=0; i < len(args); i=i+2 {
		t.add_ecert(stub, args[i], args[i+1])
	}

	return nil, nil
}




//==============================================================================================================================
//	 General Functions
//==============================================================================================================================
//	 get_ecert - Takes the name passed and calls out to the REST API for HyperLedger to retrieve the ecert
//				 for that user. Returns the ecert as retrived including html encoding.
//==============================================================================================================================
func (t *SimpleChaincode) get_ecert(stub shim.ChaincodeStubInterface, name string) ([]byte, error) {

	ecert, err := stub.GetState(name)

	if err != nil { return nil, errors.New("Couldn't retrieve ecert for user " + name) }

	return ecert, nil
}

//==============================================================================================================================
//	 add_ecert - Adds a new ecert and user pair to the table of ecerts
//==============================================================================================================================

func (t *SimpleChaincode) add_ecert(stub shim.ChaincodeStubInterface, name string, ecert string) ([]byte, error) {


	err := stub.PutState(name, []byte(ecert))

	if err == nil {
		return nil, errors.New("Error storing eCert for user " + name + " identity: " + ecert)
	}

	return nil, nil

}

//==============================================================================================================================
//	 get_caller - Retrieves the username of the user who invoked the chaincode.
//				  Returns the username as a string.
//==============================================================================================================================

func (t *SimpleChaincode) get_username(stub shim.ChaincodeStubInterface) (string, error) {

    username, err := stub.ReadCertAttribute("username");
	if err != nil { return "", errors.New("Couldn't get attribute 'username'. Error: " + err.Error()) }
	return string(username), nil
}

//==============================================================================================================================
//	 check_affiliation - Takes an ecert as a string, decodes it to remove html encoding then parses it and checks the
// 				  		certificates common name. The affiliation is stored as part of the common name.
//==============================================================================================================================

func (t *SimpleChaincode) check_affiliation(stub shim.ChaincodeStubInterface) (string, error) {
    affiliation, err := stub.ReadCertAttribute("role");
	if err != nil { return "", errors.New("Couldn't get attribute 'role'. Error: " + err.Error()) }
	return string(affiliation), nil

}

//==============================================================================================================================
//	 get_caller_data - Calls the get_ecert and check_role functions and returns the ecert and role for the
//					 name passed.
//==============================================================================================================================

func (t *SimpleChaincode) get_caller_data(stub shim.ChaincodeStubInterface) (string, string, error){

	user, err := t.get_username(stub)

    // if err != nil { return "", "", err }

	// ecert, err := t.get_ecert(stub, user);

    // if err != nil { return "", "", err }

	affiliation, err := t.check_affiliation(stub);

    if err != nil { return "", "", err }

	return user, affiliation, nil
}

//==============================================================================================================================
//	 retrieve_ILNS - Gets the state of the Member at ILNSID in the ledger then converts it from the stored
//					JSON into the Member struct for use in the contract. Returns the Member struct.
//					Returns empty m if it errors.
//==============================================================================================================================
func (t *SimpleChaincode) retrieve_ILNS(stub shim.ChaincodeStubInterface, ILNSID string) (Member, error) {

	var m Member

	bytes, err := stub.GetState(ILNSID);

	if err != nil {	fmt.Printf("RETRIEVE_ILNS: Failed to invoke member_code: %s", err); return m, errors.New("RETRIEVE_ILNS: Error retrieving member with ILNSID = " + ILNSID) }

	err = json.Unmarshal(bytes, &m);

    if err != nil {	fmt.Printf("RETRIEVE_ILNS: Corrupt member record "+string(bytes)+": %s", err); return m, errors.New("RETRIEVE_ILNS: Corrupt member record"+string(bytes))	}

	return m, nil
}



//==============================================================================================================================
// save_changes - Writes to the ledger the member struct passed in a JSON format. Uses the shim file's
//				  method 'PutState'.
//==============================================================================================================================
func (t *SimpleChaincode) save_changes(stub shim.ChaincodeStubInterface, m Member) (bool, error) {

	bytes, err := json.Marshal(m)

	if err != nil { fmt.Printf("SAVE_CHANGES: Error converting member record: %s", err); return false, errors.New("Error converting member record") }

	err = stub.PutState(m.ILNSID, bytes)

	if err != nil { fmt.Printf("SAVE_CHANGES: Error storing member record: %s", err); return false, errors.New("Error storing member record") }

	return true, nil
}



//==============================================================================================================================
//	 Router Functions
//==============================================================================================================================
//	Invoke - Called on chaincode invoke. Takes a function name passed and calls that function. Converts some
//		  initial arguments passed to other things for use in the called function e.g. name -> ecert
//==============================================================================================================================
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	caller, caller_affiliation, err := t.get_caller_data(stub)

	if err != nil { return nil, errors.New("Error retrieving caller information")}


	if function == "create_member" {
        return t.create_member(stub, caller, caller_affiliation, args[0])
	} else if function == "ping" {
        return t.ping(stub)
    } else { 																				// If the function is not a create then there must be a member so we need to retrieve the member.
		argPos := 1

		if function == "dead_member" {																// If its a dead member then only two arguments are passed (no update value) all others have three arguments and the ILNSID is expected in the last argument
			argPos = 0
		}

		m, err := t.retrieve_ILNS(stub, args[argPos])

        if err != nil { fmt.Printf("INVOKE: Error retrieving ILNS: %s", err); return nil, errors.New("Error retrieving ILNS") }


        if strings.Contains(function, "update") == false && function != "dead_member"    { 									// If the function is not an update or a death it must be a transfer so we need to get the ecert of the recipient.


				if 		   function == "parents_to_birthday" { return t.parents_to_birthday(stub, m, caller, caller_affiliation, args[0], "birthday")
				} else if  function == "birthday_to_healthy"   { return t.birthday_to_healthy(stub, m, caller, caller_affiliation, args[0], "healthy")
				} else if  function == "healthy_to_illness"  { return t.healthy_to_illness(stub, m, caller, caller_affiliation, args[0], "illness")
				} else if  function == "illness_to_illness" 	   { return t.illness_to_illness(stub, m, caller, caller_affiliation, args[0], "illness")
				} else if  function == "illness_to_healthy"  { return t.illness_to_healthy(stub, m, caller, caller_affiliation, args[0], "healthy")
				} else if  function == "healthy_to_death" { return t.healthy_to_death(stub, m, caller, caller_affiliation, args[0], "death")
				} else if  function == "illness_to_death" { return t.illness_to_death(stub, m, caller, caller_affiliation, args[0], "death")
				}

		} else if function == "update_DOB"        	{ return t.update_DOB(stub, m, caller, caller_affiliation, args[0])
		} else if function == "update_gender" 		{ return t.update_gender(stub, m, caller, caller_affiliation, args[0])
		} else if function == "update_BloodGrp" 	{ return t.update_BloodGrp(stub, m, caller, caller_affiliation, args[0])
        	} else if function == "update_Weight" 		{ return t.update_Weight(stub, m, caller, caller_affiliation, args[0])
		} else if function == "dead_member" 		{ return t.dead_member(stub, m, caller, caller_affiliation) }

		return nil, errors.New("Function of the name "+ function +" doesn't exist.")

	}
}


//=================================================================================================================================
//	Query - Called on chaincode query. Takes a function name passed and calls that function. Passes the
//  		initial arguments passed are passed on to the called function.
//=================================================================================================================================
func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	caller, caller_affiliation, err := t.get_caller_data(stub)
	if err != nil { fmt.Printf("QUERY: Error retrieving caller details", err); return nil, errors.New("QUERY: Error retrieving caller details: "+err.Error()) }

    logger.Debug("function: ", function)
    logger.Debug("caller: ", caller)
    logger.Debug("affiliation: ", caller_affiliation)

	if function == "get_member_details" {
		if len(args) != 1 { fmt.Printf("Incorrect number of arguments passed"); return nil, errors.New("QUERY: Incorrect number of arguments passed") }
		m, err := t.retrieve_ILNS(stub, args[0])
		if err != nil { fmt.Printf("QUERY: Error retrieving ILNS: %s", err); return nil, errors.New("QUERY: Error retrieving ILNS "+err.Error()) }
		return t.get_member_details(stub, m, caller, caller_affiliation)
	} else if function == "check_unique_ILNS" {
		return t.check_unique_ILNS(stub, args[0], caller, caller_affiliation)
	} else if function == "get_members" {
		return t.get_members(stub, caller, caller_affiliation)
	} else if function == "get_ecert" {
		return t.get_ecert(stub, args[0])
	} else if function == "ping" {
		return t.ping(stub)
	}

	return nil, errors.New("Received unknown function invocation " + function)

}


//=================================================================================================================================
//	 Ping Function
//=================================================================================================================================
//	 Pings the peer to keep the connection alive
//=================================================================================================================================
func (t *SimpleChaincode) ping(stub shim.ChaincodeStubInterface) ([]byte, error) {
	return []byte("Hello, world!"), nil
}



//=================================================================================================================================
//	 Create Function
//=================================================================================================================================
//	 Create member - Creates the initial JSON for the vehcile and then saves it to the ledger.
//=================================================================================================================================
func (t *SimpleChaincode) create_member(stub shim.ChaincodeStubInterface, caller string, caller_affiliation string, ILNSID string) ([]byte, error) {
	var m Member

	ILNS_ID         := "\"ILNSID\":\""+ILNSID+"\", "							// Variables to define the JSON
	name         	:= "\"Name\":\""+caller+"\", "
	DOB          	:= "\"DOB\":\"UNDEFINED\", "
	gender          := "\"Gender\":\"UNDEFINED\", "
	BloodGrp       := "\"BloodGrp\":\"UNDEFINED\", "
	Weight          := "\"Weight\":0, "
	IllnessID	:= "\"ILNSID\":\"UNDEFINED\", "
	status          := "\"Status\":0, "
	dead       	:= "\"Dead\":false"

	member_json := "{"+ILNS_ID+name+DOB+gender+BloodGrp+Weight+IllnessID+status+dead+"}" 	// Concatenates the variables to create the total JSON object

	matched, err := regexp.Match("^[A-z][A-z][0-8]{7}", []byte(ILNSID))  				// matched = true if the ILNSID passed fits format of two letters followed by seven digits

	if err != nil { fmt.Printf("CREATE_member: Invalid ILNSID: %s", err); return nil, errors.New("Invalid ILNSID") }

	if 				ILNS_ID  == "" 	 ||
					matched == false    {
	fmt.Printf("CREATE_MEMBER: Invalid ILNSID provided");
	return nil, errors.New("Invalid ILNSID provided")
	}

	err = json.Unmarshal([]byte(member_json), &m)							// Convert the JSON defined above into a member object for go

	if err != nil { return nil, errors.New("Invalid JSON object") }

	record, err := stub.GetState(m.ILNSID) 								// If not an error then a record exists so cant create a new member with this ILNSID as it must be unique

	if record != nil { return nil, errors.New("member already exists") }

	if 	caller_affiliation != PARENTS {								// Only the parents can create a new ILNS

		return nil, errors.New(fmt.Sprintf("Permission Denied. create_member. %m === %m", caller_affiliation, PARENTS))

	}

	_, err  = t.save_changes(stub, m)

	if err != nil { fmt.Printf("CREATE_MEMBER: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	bytes, err := stub.GetState("ILNSIDs")

	if err != nil { return nil, errors.New("Unable to get ILNSIDs") }

	var ILNSIDs ILNS_Holder

	err = json.Unmarshal(bytes, &ILNSIDs)

	if err != nil {	return nil, errors.New("Corrupt ILNS_Holder record") }

	ILNSIDs.ILNSs = append(ILNSIDs.ILNSs, ILNSID)


	bytes, err = json.Marshal(ILNSIDs)

	if err != nil { fmt.Print("Error creating ILNS_Holder record") }

	err = stub.PutState("ILNSIDs", bytes)

	if err != nil { return nil, errors.New("Unable to put the state") }

	return nil, nil

}



//=================================================================================================================================
//	 Transfer Functions
//=================================================================================================================================
//	 parents_to_birthday
//=================================================================================================================================
func (t *SimpleChaincode) parents_to_birthday(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, recipient_name string, recipient_affiliation string) ([]byte, error) {

	if     	m.Status				== STATE_CARRYING	&&
			m.Name				== caller		&&
			caller_affiliation		== PARENTS		&&
			recipient_affiliation		== BIRTHDAY		&&
			m.Dead				== false			{		// If the roles and users are ok

					m.Name  = recipient_name					// then make the owner the new owner
					m.Status = STATE_CARRYING						// and mark it in the state of manufacture

	} else {											// Otherwise if there is an error
			fmt.Printf("PARENTS_TO_BIRTHDAY: Permission Denied");
                        return nil, errors.New(fmt.Sprintf("Permission Denied. parents_to_birthday. %m %m === %m, %m === %m, %m === %m, %m === %m, %m === %m", m, m.Status, STATE_HEALTHY, m.Name, caller, caller_affiliation, HEALTHY, recipient_affiliation, DEATH, m.Dead, false))


	}

	_, err := t.save_changes(stub, m)						// Write new state

	if err != nil {	fmt.Printf("PARENTS_TO_BIRTHDAY: Error saving changes: %s", err); return nil, errors.New("Error saving changes")	}

	return nil, nil									// We are Done

}


//=================================================================================================================================
//	 birthday_to_healthy
//=================================================================================================================================
func (t *SimpleChaincode) birthday_to_healthy(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, recipient_name string, recipient_affiliation string) ([]byte, error) {

	if 		m.DOB 	   == "UNDEFINED" ||
			m.Gender   == "UNDEFINED" ||
			m.BloodGrp == "UNDEFINED" ||
			m.Weight      == 0				{					//If any detail of the member is undefined it has not been fully updated so cannot be sent
															fmt.Printf("BIRTHDAY_TO_HEALTHY: Member not fully defined")
															return nil, errors.New(fmt.Sprintf("Member not fully defined. %m", m))
	}

	if 		m.Status		== STATE_BIRTH		&&
			m.Name			== caller		&&
			caller_affiliation	== BIRTHDAY		&&
			recipient_affiliation	== HEALTHY		&&
			m.Dead     		== false			{

					m.Name = recipient_name
					m.Status = STATE_HEALTHY

	} else {
        return nil, errors.New(fmt.Sprintf("Permission Denied. birthday_to_healthy. %m %m === %m, %m === %m, %m === %m, %m === %m, %m === %m", m, m.Status, STATE_HEALTHY, m.Name, caller, caller_affiliation, HEALTHY, recipient_affiliation, DEATH, m.Dead, false))
    }

	_, err := t.save_changes(stub, m)

	if err != nil { fmt.Printf("BIRTHDAY_TO_HEALTHY: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}



//=================================================================================================================================
//	 healthy_to_illness
//=================================================================================================================================
func (t *SimpleChaincode) healthy_to_illness(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, recipient_name string, recipient_affiliation string) ([]byte, error) {

	if 		m.Status		== STATE_HEALTHY	&&
			m.Name			== caller		&&
			caller_affiliation	== HEALTHY		&&
			recipient_affiliation	== ILLNESS		&&
            		m.Dead     		== false			{

					m.Name = recipient_name

	} else {
        return nil, errors.New(fmt.Sprintf("Permission denied. healthy_to_illness. %m === %m, %m === %m, %m === %m, %m === %m, %m === %m", m.Status, STATE_HEALTHY, m.Name, caller, caller_affiliation, HEALTHY, recipient_affiliation, DEATH, m.Dead, false))

	}

	_, err := t.save_changes(stub, m)
	if err != nil { fmt.Printf("HEALTHY_TO_ILLNESS: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}


//=================================================================================================================================
//	 illness_to_illness
//=================================================================================================================================
func (t *SimpleChaincode) illness_to_illness(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, recipient_name string, recipient_affiliation string) ([]byte, error) {

	if 		m.Status		== STATE_ILLNESS	&&
			m.Name			== caller		&&
			caller_affiliation	== ILLNESS		&&
			recipient_affiliation	== ILLNESS		&&
			m.Dead			== false			{

					m.Name = recipient_name

	} else {
        return nil, errors.New(fmt.Sprintf("Permission Denied. illness_to_illness. %m %m === %m, %m === %m, %m === %m, %m === %m, %m === %m", m, m.Status, STATE_ILLNESS, m.Name, caller, caller_affiliation, ILLNESS, recipient_affiliation, DEATH, m.Dead, false))
	}

	_, err := t.save_changes(stub, m)

	if err != nil { fmt.Printf("ILLNESS_TO_ILLNESS: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}


//=================================================================================================================================
//	 illness_to_healthy
//=================================================================================================================================
func (t *SimpleChaincode) illness_to_healthy(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, recipient_name string, recipient_affiliation string) ([]byte, error) {

	if		m.Status		== STATE_ILLNESS	&&
			m.Name  		== caller		&&
			caller_affiliation	== HEALTHY		&&
			recipient_affiliation	== ILLNESS		&&
			m.Dead			== false			{

				m.Name = recipient_name

	} else {
		return nil, errors.New(fmt.Sprintf("Permission Denied. illness_to_healthy. %m %m === %m, %m === %m, %m === %m, %m === %m, %m === %m", m, m.Status, STATE_ILLNESS, m.Name, caller, caller_affiliation, ILLNESS, recipient_affiliation, DEATH, m.Dead, false))
	}

	_, err := t.save_changes(stub, m)
	if err != nil { fmt.Printf("ILLNESS_TO_HEALTHY: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}


//=================================================================================================================================
//	 healthy_to_death
//=================================================================================================================================
func (t *SimpleChaincode) healthy_to_death(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, recipient_name string, recipient_affiliation string) ([]byte, error) {

	if		m.Status		== STATE_HEALTHY	&&
			m.Name			== caller		&&
			caller_affiliation	== HEALTHY		&&
			recipient_affiliation	== DEATH		&&
			m.Dead			== false			{

					m.Name = recipient_name
					m.Status = STATE_DEATH

	} else {
        return nil, errors.New(fmt.Sprintf("Permission Denied. healthy_to_death. %m %m === %m, %m === %m, %m === %m, %m === %m, %m === %m", m, m.Status, STATE_HEALTHY, m.Name, caller, caller_affiliation, HEALTHY, recipient_affiliation, DEATH, m.Dead, false))
	}

	_, err := t.save_changes(stub, m)

	if err != nil { fmt.Printf("HEALTHY_TO_DEATH: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}


//=================================================================================================================================
//	 illness_to_death
//=================================================================================================================================
func (t *SimpleChaincode) illness_to_death(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, recipient_name string, recipient_affiliation string) ([]byte, error) {

	if		m.Status		== STATE_ILLNESS	&&
			m.Name			== caller		&&
			caller_affiliation	== ILLNESS		&&
			recipient_affiliation	== DEATH		&&
			m.Dead			== false			{

					m.Name = recipient_name
					m.Status = STATE_DEATH

	} else {
        return nil, errors.New(fmt.Sprintf("Permission Denied. illness_to_death. %m %m === %m, %m === %m, %m === %m, %m === %m, %m === %m", m, m.Status, STATE_ILLNESS, m.Name, caller, caller_affiliation, ILLNESS, recipient_affiliation, DEATH, m.Dead, false))
	}

	_, err := t.save_changes(stub, m)

	if err != nil { fmt.Printf("ILLNESS_TO_DEATH: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}


//=================================================================================================================================
//	 Update Functions
//=================================================================================================================================
//	 update_DOB
//=================================================================================================================================
func (t *SimpleChaincode) update_DOB(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, new_value string) ([]byte, error) {

	if 		m.Name			== caller			&&
			caller_affiliation	== BIRTHDAY			&&/*((m.Name				== caller			&&
			caller_affiliation	== BIRTHDAY)		||
			caller_affiliation	== PARENTS)			&&*/
			m.Dead			== false				{

					m.DOB = new_value
	} else {

		return nil, errors.New(fmt.Sprint("Permission denied. update_DOB %t %t %t" + m.Name == caller, caller_affiliation == BIRTHDAY, m.Dead))
	}

	_, err := t.save_changes(stub, m)

		if err != nil { fmt.Printf("UPDATE_DOB: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}


//=================================================================================================================================
//	 update_BloodGrp
//=================================================================================================================================
func (t *SimpleChaincode) update_BloodGrp(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, new_value string) ([]byte, error) {


	if		m.Name			== caller	&&
			caller_affiliation	== BIRTHDAY	&&
			m.Dead			== false		{

					m.BloodGrp = new_value

	} else {
        return nil, errors.New(fmt.Sprint("Permission denied. update_BloodGrp"))
	}

	_, err := t.save_changes(stub, m)

	if err != nil { fmt.Printf("UPDATE_BloodGrp: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}

//=================================================================================================================================
//	 update_gender
//=================================================================================================================================
func (t *SimpleChaincode) update_gender(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, new_value string) ([]byte, error) {


	if		m.Name			== caller	&&
			caller_affiliation	!= DEATH	&&
			m.Dead			== false		{

					m.Gender = new_value

	} else {
        return nil, errors.New(fmt.Sprint("Permission denied. update_gender"))
	}

	_, err := t.save_changes(stub, m)

	if err != nil { fmt.Printf("UPDATE_GENDER: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}


//=================================================================================================================================
//	 update_Weight
//=================================================================================================================================
func (t *SimpleChaincode) update_Weight(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string, new_value string) ([]byte, error) {

	new_Weight, err := strconv.Atoi(string(new_value)) 		                // will return an error if the new vin contains non numerical chars

	if err != nil || len(string(new_value)) != 15 { return nil, errors.New("Invalid value passed for new Weight") }

	if 		m.Name			== caller		&&
			caller_affiliation	!= DEATH		&&
			m.Dead			== false			{

					m.Weight = new_Weight					// Update to the new value
	} else {

        return nil, errors.New(fmt.Sprintf("Permission denied. update_Weight %m %m %m %m %m", m.Status, STATE_BIRTH, m.Name, caller, m.Weight, m.Dead))

	}

	_, err  = t.save_changes(stub, m)						// Save the changes in the blockchain

	if err != nil { fmt.Printf("UPDATE_WEIGHT: Error saving changes: %s", err); return nil, errors.New("Error saving changes") }

	return nil, nil

}


//=================================================================================================================================
//	 dead_member
//=================================================================================================================================
func (t *SimpleChaincode) dead_member(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string) ([]byte, error) {

	if		m.Status		== STATE_DEATH		&&
			m.Name			== caller		&&
			caller_affiliation	== DEATH		&&
			m.Dead			== false				{

					m.Dead = true

	} else {
		return nil, errors.New("Permission denied. dead_member")
	}

	_, err := t.save_changes(stub, m)

	if err != nil { fmt.Printf("DEAD_MEMBER: Error saving changes: %s", err); return nil, errors.New("DEAD_MEMBER error saving changes") }

	return nil, nil

}


//=================================================================================================================================
//	 Read Functions
//=================================================================================================================================
//	 get_member_details
//=================================================================================================================================
func (t *SimpleChaincode) get_member_details(stub shim.ChaincodeStubInterface, m Member, caller string, caller_affiliation string) ([]byte, error) {

	bytes, err := json.Marshal(m)

	if err != nil { return nil, errors.New("GET_MEMBER_DETAILS: Invalid member object") }

	if 		m.Name			== caller		||
			caller_affiliation	== PARENTS	{

					return bytes, nil
	} else {
			return nil, errors.New("Permission Denied. get_mobile_details")
	}

}


//=================================================================================================================================
//	 get_members
//=================================================================================================================================

func (t *SimpleChaincode) get_members(stub shim.ChaincodeStubInterface, caller string, caller_affiliation string) ([]byte, error) {
	bytes, err := stub.GetState("ILNSIDs")

	if err != nil { return nil, errors.New("Unable to get ILNSIDs") }

	var ILNSIDs ILNS_Holder

	err = json.Unmarshal(bytes, &ILNSIDs)

	if err != nil {	return nil, errors.New("Corrupt ILNS_Holder") }

	result := "["

	var temp []byte
	var m Member

	for _, ILNS := range ILNSIDs.ILNSs {

		m, err = t.retrieve_ILNS(stub, ILNS)

		if err != nil {return nil, errors.New("Failed to retrieve ILNS")}

		temp, err = t.get_member_details(stub, m, caller, caller_affiliation)

		if err == nil {
			result += string(temp) + ","
		}
	}

	if len(result) == 1 {
		result = "[]"
	} else {
		result = result[:len(result)-1] + "]"
	}

	return []byte(result), nil
}



//=================================================================================================================================
//	 check_unique_ILNS
//=================================================================================================================================
func (t *SimpleChaincode) check_unique_ILNS(stub shim.ChaincodeStubInterface, ILNS string, caller string, caller_affiliation string) ([]byte, error) {
	_, err := t.retrieve_ILNS(stub, ILNS)
	if err == nil {
		return []byte("false"), errors.New("ILNS is not unique")
	} else {
		return []byte("true"), nil
	}
}

//=================================================================================================================================
//	 Main - main - Starts up the chaincode
//=================================================================================================================================
func main() {

	err := shim.Start(new(SimpleChaincode))

	if err != nil { fmt.Printf("Error starting Chaincode: %s", err) }
}



