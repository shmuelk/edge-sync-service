package base

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/open-horizon/edge-sync-service/core/security"

	"github.com/open-horizon/edge-sync-service/common"
	"github.com/open-horizon/edge-sync-service/core/communications"
	"github.com/open-horizon/edge-utilities/logger"
	"github.com/open-horizon/edge-utilities/logger/log"
	"github.com/open-horizon/edge-utilities/logger/trace"
)

const destinationsURL = "/api/v1/destinations"
const objectsURL = "/api/v1/objects/"
const organizationURL = "/api/v1/organizations/"
const getOrganizationsURL = "/api/v1/organizations"
const resendURL = "/api/v1/resend"
const securityURL = "/api/v1/security/"
const shutdownURL = "/api/v1/shutdown"

const (
	contentType     = "Content-Type"
	applicationJSON = "application/json"
)

var unauthorizedBytes = []byte("Unauthorized")

// objectUpdate includes the object's metadata and data
// A sync service object includes metadata and optionally binary data.
// When an object is created the metadata must be provided. The metadata and the data can then be updated together or one at a time.
// swagger:model
type objectUpdate struct {
	// Meta is the object's metadata
	Meta common.MetaData `json:"meta"`

	// Data is a the object's binary data
	Data []byte `json:"data"`
}

// webhookUpdate includes the webhook's action and URL
// A webhook can be used to allow the sync service to invoke actions when new information becomes available.
// An application can choose between using a webhook and periodically polling the sync service for updates.
// swagger:model
type webhookUpdate struct {
	// Action is an action can be either register (create/update a webhook) or delete (delete the webhook)
	Action string `json:"action"`

	// URL is the URL to invoke when new information for the object is available
	URL string `json:"url"`
}

// organization includes the organization's id and broker address
// swagger:model
type organization struct {
	// Organization OD
	OrgID string `json:"org-id"`

	// Broker address
	Address string `json:"address"`
}

// bulkACLUpdate is the payload used when performing a bulk update on an ACL (either adding usernames to an
// ACL or removing usernames from an ACL.
// swagger:model
type bulkACLUpdate struct {
	// Action is an action, which can be either add (to add usernames) or remove (to remove usernames)
	Action string `json:"action"`

	// Usernames is an array of usernames to be added or removed from the ACL as appropriate
	Usernames []string `json:"usernames"`
}

func setupAPIServer() {
	if common.Configuration.NodeType == common.CSS {
		http.Handle(destinationsURL+"/", http.StripPrefix(destinationsURL+"/", http.HandlerFunc(handleDestinations)))
		http.Handle(securityURL, http.StripPrefix(securityURL, http.HandlerFunc(handleSecurity)))
	} else {
		http.HandleFunc(destinationsURL, handleDestinations)
	}
	http.Handle(objectsURL, http.StripPrefix(objectsURL, http.HandlerFunc(handleObjects)))
	http.HandleFunc(shutdownURL, handleShutdown)
	http.HandleFunc(resendURL, handleResend)
	http.Handle(getOrganizationsURL, http.StripPrefix(getOrganizationsURL, http.HandlerFunc(handleGetOrganizations)))
	http.Handle(organizationURL, http.StripPrefix(organizationURL, http.HandlerFunc(handleOrganizations)))
}

// swagger:operation GET /api/v1/destinations/{orgID} handleDestinations
//
// List all known destinations.
//
// Provides a list of destinations for an organization, i.e., ESS nodes (belonging to orgID) that have registered with the CSS.
// This is a CSS only API.
//
// ---
//
// produces:
// - application/json
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the destinations to return.
//   required: true
//   type: string
//
// responses:
//   '200':
//     description: Destinations response
//     schema:
//       type: array
//       items:
//         "$ref": "#/definitions/Destination"
//   '404':
//     description: No destinations found
//     schema:
//       type: string
//   '500':
//     description: Failed to retrieve the destinations
//     schema:
//       type: string
func handleDestinations(writer http.ResponseWriter, request *http.Request) {
	if !common.Running {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	username, password, ok := request.BasicAuth()
	if !ok {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}
	code, userOrg, _ := security.Authenticate(username, password)
	if code == security.AuthFailed || code == security.AuthEdgeNode {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}

	if request.Method == http.MethodGet {
		var orgID string
		if common.Configuration.NodeType == common.CSS {
			if len(request.URL.Path) == 0 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			orgID = request.URL.Path
		} else {
			orgID = common.Configuration.OrgID
		}

		if userOrg != orgID && code != security.AuthSyncAdmin {
			writer.WriteHeader(http.StatusForbidden)
			writer.Write(unauthorizedBytes)
			return
		}

		if dests, err := listDestinations(orgID); err != nil {
			communications.SendErrorResponse(writer, err, "Failed to fetch the list of destinations. Error: ", 0)
		} else {
			if len(dests) == 0 {
				writer.WriteHeader(http.StatusNotFound)
			} else {
				if data, err := json.MarshalIndent(dests, "", "  "); err != nil {
					communications.SendErrorResponse(writer, err, "Failed to marshal the list of destinations. Error: ", 0)
				} else {
					writer.Header().Add(contentType, applicationJSON)
					writer.WriteHeader(http.StatusOK)
					writer.Write(data)
				}
			}
		}
	} else {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// swagger:operation POST /api/v1/resend handleResend
//
// Request to resend objects.
//
// Used by an ESS to ask the CSS to resend it all the objects (supported only for ESS to CSS requests).
// An application only needs to use this API in case the data it previously obtained from the ESS was lost.
//
// ---
//
// produces:
// - text/plain
//
// responses:
//   '204':
//     description: The request will be sent
//     schema:
//       type: string
//   '400':
//     description: The request is not allowed on Cloud Sync-Service
//     schema:
//       type: string
func handleResend(writer http.ResponseWriter, request *http.Request) {
	if !common.Running {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	username, password, ok := request.BasicAuth()
	if !ok {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}
	code, _, _ := security.Authenticate(username, password)
	if code != security.AuthAdmin && code != security.AuthUser {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}

	if request.Method == http.MethodPost {
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("In handleResend\n")
		}
		if err := resendObjects(); err != nil {
			communications.SendErrorResponse(writer, err, "Failed to send resend objects request. Error: ", 0)
		} else {
			writer.WriteHeader(http.StatusNoContent)
		}
	} else {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleShutdown(writer http.ResponseWriter, request *http.Request) {
	username, password, ok := request.BasicAuth()
	if !ok {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}
	code, _, _ := security.Authenticate(username, password)
	if code != security.AuthSyncAdmin {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}

	if request.Method == http.MethodPost {
		writer.WriteHeader(http.StatusNoContent)

		restart := strings.ToLower(request.URL.Query().Get("restart"))
		quiesceString := request.URL.Query().Get("quiesce")

		go func() {
			timer := time.NewTimer(time.Duration(1) * time.Second)
			<-timer.C

			quieceTime := 3
			if len(quiesceString) != 0 {
				var quieceTemp int
				_, err := fmt.Sscanf(quiesceString, "%d", &quieceTemp)
				if err == nil {
					quieceTime = quieceTemp
				}
			}

			if restart == "true" || restart == "yes" {
				// If BlockUntilShutdown was called, don't let Stop() unblock
				blocking := waitingOnBlockChannel
				waitingOnBlockChannel = false
				Stop(quieceTime)

				if log.IsLogging(logger.INFO) {
					log.Info("Restarting the Sync Service")
				}
				Start("", false)
				waitingOnBlockChannel = blocking
			} else {
				Stop(quieceTime)
			}
		}()
	} else {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleObjects(writer http.ResponseWriter, request *http.Request) {
	if !common.Running {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	if len(request.URL.Path) != 0 {
		parts := strings.Split(request.URL.Path, "/")
		var orgID string
		if common.Configuration.NodeType == common.CSS {
			if len(parts) == 1 {
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			orgID = parts[0]
			parts = parts[1:]
		} else {
			orgID = common.Configuration.OrgID
		}

		if len(parts) == 1 || (len(parts) == 2 && len(parts[1]) == 0) {
			// /api/v1/objects/orgID/type
			// GET - get updated objects
			// PUT - register/delete a webhook
			switch request.Method {
			case http.MethodGet:
				receivedString := request.URL.Query().Get("received")
				received := false
				if receivedString != "" {
					var err error
					received, err = strconv.ParseBool(receivedString)
					if err != nil {
						writer.WriteHeader(http.StatusBadRequest)
						return
					}
				}
				handleListUpdatedObjects(orgID, parts[0], received, writer, request)
			case http.MethodPut:
				handleWebhook(orgID, parts[0], writer, request)
			default:
				writer.WriteHeader(http.StatusMethodNotAllowed)
			}

		} else if len(parts) == 2 || (len(parts) == 3 && len(parts[2]) == 0) {
			// GET/DELETE/PUT /api/v1/objects/orgID/type/id
			handleObjectRequest(orgID, parts[0], parts[1], writer, request)

		} else if len(parts) == 3 || (len(parts) == 4 && len(parts[3]) == 0) {
			// PUT     /api/v1/objects/orgID/type/id/consumed
			// PUT     /api/v1/objects/orgID/type/id/deleted
			// PUT     /api/v1/objects/orgID/type/id/received
			// PUT     /api/v1/objects/orgID/type/id/activate
			// GET     /api/v1/objects/orgID/type/id/status
			// GET/PUT /api/v1/objects/orgID/type/id/data
			// GET     /api/v1/objects/orgID/type/id/destinations
			operation := strings.ToLower(parts[2])
			handleObjectOperation(operation, orgID, parts[0], parts[1], writer, request)

		} else {
			writer.WriteHeader(http.StatusBadRequest)
		}
	} else {
		writer.WriteHeader(http.StatusBadRequest)
	}
}

func handleObjectRequest(orgID string, objectType string, objectID string, writer http.ResponseWriter, request *http.Request) {
	switch request.Method {

	// swagger:operation GET /api/v1/objects/{orgID}/{objectType}/{objectID} handleGetObject
	//
	// Get an object.
	//
	// Get the metadata of an object of the specified object type and object ID.
	// The metadata indicates if the objects includes data which can then be obtained using the appropriate API.
	//
	// ---
	//
	// produces:
	// - application/json
	// - text/plain
	//
	// parameters:
	// - name: orgID
	//   in: path
	//   description: The orgID of the object to return. Present only when working with a CSS, removed from the path when working with an ESS
	//   required: true
	//   type: string
	// - name: objectType
	//   in: path
	//   description: The object type of the object to return
	//   required: true
	//   type: string
	// - name: objectID
	//   in: path
	//   description: The object ID of the object to return
	//   required: true
	//   type: string
	//
	// responses:
	//   '200':
	//     description: Object response
	//     schema:
	//       "$ref": "#/definitions/MetaData"
	//   '404':
	//     description: Object not found
	//     schema:
	//       type: string
	//   '500':
	//     description: Failed to retrieve the object
	//     schema:
	//       type: string
	case http.MethodGet:
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("In handleObjects. Get %s %s\n", objectType, objectID)
		}
		if !canUserAccessObject(request, orgID, objectType) {
			writer.WriteHeader(http.StatusForbidden)
			writer.Write(unauthorizedBytes)
			return
		}
		if metaData, err := getObject(orgID, objectType, objectID); err != nil {
			communications.SendErrorResponse(writer, err, "", 0)
		} else {
			if metaData == nil {
				writer.WriteHeader(http.StatusNotFound)
			} else {
				if data, err := json.MarshalIndent(metaData, "", "  "); err != nil {
					communications.SendErrorResponse(writer, err, "Failed to marshal metadata. Error: ", 0)
				} else {
					writer.Header().Add(contentType, applicationJSON)
					writer.WriteHeader(http.StatusOK)
					writer.Write(data)
				}
			}
		}

	// swagger:operation DELETE /api/v1/objects/{orgID}/{objectType}/{objectID} handleDeleteObject
	//
	// Delete an object.
	//
	// Delete the object of the specified object type and object ID.
	// Destinations of the object will be notified that the object has been deleted.
	//
	// ---
	//
	// produces:
	// - text/plain
	//
	// parameters:
	// - name: orgID
	//   in: path
	//   description: The orgID of the object to delete. Present only when working with a CSS, removed from the path when working with an ESS
	//   required: true
	//   type: string
	// - name: objectType
	//   in: path
	//   description: The object type of the object to delete
	//   required: true
	//   type: string
	// - name: objectID
	//   in: path
	//   description: The object ID of the object to delete
	//   required: true
	//   type: string
	//
	// responses:
	//   '204':
	//     description: Object deleted
	//     schema:
	//       type: string
	//   '500':
	//     description: Failed to delete the object
	//     schema:
	//       type: string
	case http.MethodDelete:
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("In handleObjects. Delete %s %s\n", objectType, objectID)
		}
		if !canUserAccessObject(request, orgID, objectType) {
			writer.WriteHeader(http.StatusForbidden)
			writer.Write(unauthorizedBytes)
			return
		}
		if err := deleteObject(orgID, objectType, objectID); err != nil {
			communications.SendErrorResponse(writer, err, "Failed to delete the object. Error: ", 0)
		} else {
			writer.WriteHeader(http.StatusNoContent)
		}

	case http.MethodPut:
		handleUpdateObject(orgID, objectType, objectID, writer, request)

	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleObjectOperation(operation string, orgID string, objectType string, objectID string, writer http.ResponseWriter, request *http.Request) {
	if !canUserAccessObject(request, orgID, objectType) {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}
	switch operation {
	case "consumed":
		handleObjectConsumed(orgID, objectType, objectID, writer, request)
	case "deleted":
		handleObjectDeleted(orgID, objectType, objectID, writer, request)
	case "received":
		handleObjectReceived(orgID, objectType, objectID, writer, request)
	case "activate":
		handleActivateObject(orgID, objectType, objectID, writer, request)
	case "status":
		handleObjectStatus(orgID, objectType, objectID, writer, request)
	case "destinations":
		handleObjectDestinations(orgID, objectType, objectID, writer, request)
	case "data":
		switch request.Method {
		case http.MethodGet:
			handleObjectGetData(orgID, objectType, objectID, writer)

		case http.MethodPut:
			handleObjectPutData(orgID, objectType, objectID, writer, request)

		default:
			writer.WriteHeader(http.StatusMethodNotAllowed)
		}
	default:
		writer.WriteHeader(http.StatusBadRequest)
	}
}

// swagger:operation PUT /api/v1/objects/{orgID}/{objectType}/{objectID}/consumed handleObjectConsumed
//
// Mark an object as consumed.
//
// Mark the object of the specified object type and object ID as having been consumed by the application.
// After the object is marked as consumed it will not be delivered to the application again (even if the sync service is restarted).
//
// ---
//
// produces:
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the object to mark as consumed. Present only when working with a CSS, removed from the path when working with an ESS
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the object to mark as consumed
//   required: true
//   type: string
// - name: objectID
//   in: path
//   description: The object ID of the object to mark as consumed
//   required: true
//   type: string
//
// responses:
//   '204':
//     description: Object marked as consumed
//     schema:
//       type: string
//   '500':
//     description: Failed to mark the object consumed
//     schema:
//       type: string
func handleObjectConsumed(orgID string, objectType string, objectID string, writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPut {
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("In handleObjects. Consumed %s %s\n", objectType, objectID)
		}
		if err := objectConsumed(orgID, objectType, objectID); err != nil {
			communications.SendErrorResponse(writer, err, "Failed to mark the object as consumed. Error: ", 0)
		} else {
			writer.WriteHeader(http.StatusNoContent)
		}
	} else {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// swagger:operation PUT /api/v1/objects/{orgID}/{objectType}/{objectID}/deleted handleObjectDeleted
//
// The service confirms object deletion.
//
// Confirm the deletion of the object of the specified object type and object ID by the application.
// The application should invoke this API after it completed the actions associated with deleting the object.
//
// ---
//
// produces:
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the object to confirm its deletion. Present only when working with a CSS, removed from the path when working with an ESS
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the object to confirm its deletion
//   required: true
//   type: string
// - name: objectID
//   in: path
//   description: The object ID of the object to confirm its deletion
//   required: true
//   type: string
//
// responses:
//   '204':
//     description: Object's deletion confirmed
//     schema:
//       type: string
//   '500':
//     description: Failed to confirm the object's deletion
//     schema:
//       type: string
func handleObjectDeleted(orgID string, objectType string, objectID string, writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPut {
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("In handleObjects. Deleted %s %s\n", objectType, objectID)
		}
		if err := objectDeleted(orgID, objectType, objectID); err != nil {
			communications.SendErrorResponse(writer, err, "Failed to confirm object's deletion. Error: ", 0)
		} else {
			writer.WriteHeader(http.StatusNoContent)
		}
	} else {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// swagger:operation PUT /api/v1/objects/{orgID}/{objectType}/{objectID}/received handleObjectReceived
//
// Mark an object as received.
//
// Mark the object of the specified object type and object ID as having been received by the application.
// After the object is marked as received it will only be delivered to the application again if specified in the objects request.
//
// ---
//
// produces:
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the object to mark as received.
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the object to mark as received
//   required: true
//   type: string
// - name: objectID
//   in: path
//   description: The object ID of the object to mark as received
//   required: true
//   type: string
//
// responses:
//   '204':
//     description: Object marked as received
//     schema:
//       type: string
//   '500':
//     description: Failed to mark the object received
//     schema:
//       type: string
func handleObjectReceived(orgID string, objectType string, objectID string, writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPut {
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("In handleObjects. Received %s %s\n", objectType, objectID)
		}
		if err := objectReceived(orgID, objectType, objectID); err != nil {
			communications.SendErrorResponse(writer, err, "Failed to mark the object as received. Error: ", 0)
		} else {
			writer.WriteHeader(http.StatusNoContent)
		}
	} else {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// swagger:operation PUT /api/v1/objects/{orgID}/{objectType}/{objectID}/activate handleActivateObject
//
// Mark an object as active.
//
// Mark the object of the specified object type and object ID as active.
// An object can be created as inactive which means it is not delivered to its destinations.
// This API is used to activate such objects and initiate the distribution of the object to its destinations.
//
// ---
//
// produces:
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the object to mark as active. Present only when working with a CSS, removed from the path when working with an ESS
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the object to mark as active
//   required: true
//   type: string
// - name: objectID
//   in: path
//   description: The object ID of the object to mark as active
//   required: true
//   type: string
//
// responses:
//   '204':
//     description: Object marked as active
//     schema:
//       type: string
//   '500':
//     description: Failed to mark the object active
//     schema:
//       type: string
func handleActivateObject(orgID string, objectType string, objectID string, writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPut {
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("In handleObjects. Activate %s %s\n", objectType, objectID)
		}
		if err := activateObject(orgID, objectType, objectID); err != nil {
			communications.SendErrorResponse(writer, err, "Failed to mark the object as active. Error: ", 0)
		} else {
			writer.WriteHeader(http.StatusNoContent)
		}
	} else {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// swagger:operation GET /api/v1/objects/{orgID}/{objectType}/{objectID}/status handleObjectStatus
//
// Get the status of an object.
//
// Get the status of the object of the specified object type and object ID.
// The status can be one of the following:
//   notReady - The object is not ready to be sent to destinations.
//   ready - The object is ready to be sent to destinations.
//   received - The object's metadata has been received but not all its data.
//   completelyReceived - The full object (metadata and data) has been received.
//   consumed - The object has been consumed by the application.
//   deleted - The object was deleted.
//
//
// ---
//
// produces:
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the object whose status will be retrieved. Present only when working with a CSS, removed from the path when working with an ESS
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the object whose status will be retrieved
//   required: true
//   type: string
// - name: objectID
//   in: path
//   description: The object ID of the object whose status will be retrieved
//   required: true
//   type: string
//
// responses:
//   '200':
//     description: Object status
//     schema:
//       type: string
//   '500':
//     description: Failed to retrieve the object's status
//     schema:
//       type: string
func handleObjectStatus(orgID string, objectType string, objectID string, writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("In handleObjects. Get status of %s %s\n", objectType, objectID)
		}
		if status, err := getObjectStatus(orgID, objectType, objectID); err != nil {
			communications.SendErrorResponse(writer, err, "", 0)
		} else {
			if status == "" {
				writer.WriteHeader(http.StatusNotFound)
			} else {
				writer.Header().Add(contentType, "plain/text")
				writer.WriteHeader(http.StatusOK)
				writer.Write([]byte(status))
			}
		}
	} else {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// swagger:operation GET /api/v1/objects/{orgID}/{objectType}/{objectID}/destinations handleObjectDestinations
//
// Get the destinations of an object.
//
// Get the list of sync service (ESS) nodes which are the destinations of the object of the specified object type and object ID.
// The delivery status of the object is provided for each destination along with its type and ID.
// This is a CSS only API.
//
// ---
//
// produces:
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the object whose destinations will be retrieved. Present only when working with a CSS, removed from the path when working with an ESS
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the object whose destinations will be retrieved
//   required: true
//   type: string
// - name: objectID
//   in: path
//   description: The object ID of the object whose destinations will be retrieved
//   required: true
//   type: string
//
// responses:
//   '200':
//     description: Object destinations and their status
//     schema:
//       type: array
//       items:
//         "$ref": "#/definitions/DestinationsStatus"
//   '500':
//     description: Failed to retrieve the object's destinations
//     schema:
//       type: string
func handleObjectDestinations(orgID string, objectType string, objectID string, writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("In handleObjects. Get destinations of %s %s\n", objectType, objectID)
		}
		if dests, err := getObjectDestinationsStatus(orgID, objectType, objectID); err != nil {
			communications.SendErrorResponse(writer, err, "", 0)
		} else {
			if dests == nil {
				writer.WriteHeader(http.StatusNotFound)
			} else {
				if destinations, err := json.MarshalIndent(dests, "", "  "); err != nil {
					communications.SendErrorResponse(writer, err, "Failed to marshal object's destinations. Error: ", 0)
				} else {
					writer.Header().Add(contentType, applicationJSON)
					writer.WriteHeader(http.StatusOK)
					writer.Write([]byte(destinations))
				}
			}
		}
	} else {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// swagger:operation GET /api/v1/objects/{orgID}/{objectType}/{objectID}/data handleObjectGetData
//
// Get the data of an object.
//
// Get the data of the object of the specified object type and object ID.
// The metadata of the object indicates if the object includes data (noData is false).
//
// ---
//
// produces:
// - application/octet-stream
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the object whose data will be retrieved. Present only when working with a CSS, removed from the path when working with an ESS
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the object whose data will be retrieved
//   required: true
//   type: string
// - name: objectID
//   in: path
//   description: The object ID of the object whose data will be retrieved
//   required: true
//   type: string
//
// responses:
//   '200':
//     description: Object data
//     schema:
//       type: string
//       format: binary
//   '500':
//     description: Failed to retrieve the object's data
//     schema:
//       type: string
func handleObjectGetData(orgID string, objectType string, objectID string, writer http.ResponseWriter) {
	if trace.IsLogging(logger.DEBUG) {
		trace.Debug("In handleObjects. Get data %s %s\n", objectType, objectID)
	}
	if dataReader, err := getObjectData(orgID, objectType, objectID); err != nil {
		communications.SendErrorResponse(writer, err, "", 0)
	} else {
		if dataReader == nil {
			writer.WriteHeader(http.StatusNotFound)
		} else {
			writer.Header().Add(contentType, "application/octet-stream")
			writer.WriteHeader(http.StatusOK)
			if _, err := io.Copy(writer, dataReader); err != nil {
				communications.SendErrorResponse(writer, err, "", 0)
			}
			if err := store.CloseDataReader(dataReader); err != nil {
				communications.SendErrorResponse(writer, err, "", 0)
			}
		}
	}
}

// swagger:operation PUT /api/v1/objects/{orgID}/{objectType}/{objectID}/data handleObjectPutData
//
// Update the data of an object.
//
// Update the data of the object of the specified object type and object ID.
// The data can be updated without modifying the object's metadata.
//
// ---
//
// consumes:
// - application/octet-stream
//
// produces:
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the object whose data will be updated. Present only when working with a CSS, removed from the path when working with an ESS
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the object whose data will be updated
//   required: true
//   type: string
// - name: objectID
//   in: path
//   description: The object ID of the object whose data will be updated
//   required: true
//   type: string
// - name: payload
//   in: body
//   description: The object's new data
//   required: true
//   schema:
//     type: string
//     format: binary
//
// responses:
//   '200':
//     description: Object data updated
//     schema:
//       type: string
//   '404':
//     description: The specified object doesn't exist
//     schema:
//       type: string
//   '500':
//     description: Failed to update the object's data
//     schema:
//       type: string
func handleObjectPutData(orgID string, objectType string, objectID string, writer http.ResponseWriter, request *http.Request) {
	if trace.IsLogging(logger.DEBUG) {
		trace.Debug("In handleObjects. Update data %s %s\n", objectType, objectID)
	}
	if found, err := putObjectData(orgID, objectType, objectID, request.Body); err == nil {
		if !found {
			writer.WriteHeader(http.StatusNotFound)
		} else {
			writer.WriteHeader(http.StatusOK)
		}
	} else {
		communications.SendErrorResponse(writer, err, "", 0)
	}
}

// swagger:operation GET /api/v1/objects/{orgID}/{objectType}?received=bool handleListUpdatedObjects
//
// Get updated objects.
//
// Get the list of objects of the specified object type that have pending (unconsumed) updates.
// An application would typically invoke this API periodically to check for updates (an alternative is to use a webhook).
//
// ---
//
// produces:
// - application/json
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the updated objects to return. Present only when working with a CSS, removed from the path when working with an ESS
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the updated objects to return
//   required: true
//   type: string
// - name: received
//   in: query
//   description: Whether or not to include the objects that have been marked as received by the application
//   required: false
//   type: boolean
//
// responses:
//   '200':
//     description: Updated objects response
//     schema:
//       type: array
//       items:
//         "$ref": "#/definitions/MetaData"
//   '404':
//     description: No updated objects found
//     schema:
//       type: string
//   '500':
//     description: Failed to retrieve the updated objects
//     schema:
//       type: string
func handleListUpdatedObjects(orgID string, objectType string, received bool, writer http.ResponseWriter,
	request *http.Request) {
	if trace.IsLogging(logger.DEBUG) {
		trace.Debug("In handleObjects. List %s, Method %s, orgID %s, objectType %s. Include received %t\n",
			objectType, request.Method, orgID, objectType, received)
	}
	if !canUserAccessObject(request, orgID, objectType) {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}
	if metaData, err := listUpdatedObjects(orgID, objectType, received); err != nil {
		communications.SendErrorResponse(writer, err, "Failed to fetch the list of updates. Error: ", 0)
	} else {
		if len(metaData) == 0 {
			writer.WriteHeader(http.StatusNotFound)
		} else {
			if data, err := json.MarshalIndent(metaData, "", "  "); err != nil {
				communications.SendErrorResponse(writer, err, "Failed to marshal the list of updates. Error: ", 0)
			} else {
				writer.Header().Add(contentType, applicationJSON)
				writer.WriteHeader(http.StatusOK)
				writer.Write(data)
			}
		}
	}
}

// swagger:operation PUT /api/v1/objects/{orgID}/{objectType} handleWebhook
//
// Register or delete a webhook.
//
// Register or delete a webhook for the specified object type.
// A webhook is used to process notifications on updates for objects of the specified object type.
//
// ---
//
// consumes:
// - application/json
//
// produces:
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the objects for the webhook. Present only when working with a CSS, removed from the path when working with an ESS
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the objects for the webhook
//   required: true
//   type: string
// - name: payload
//   in: body
//   description: The webhook's data
//   required: true
//   schema:
//     "$ref": "#/definitions/webhookUpdate"
//
// responses:
//   '200':
//     description: Webhook registered/deleted
//     schema:
//       type: string
//   '500':
//     description: Failed to update the webhook's data
//     schema:
//       type: string
func handleWebhook(orgID string, objectType string, writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if !canUserAccessObject(request, orgID, objectType) {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}

	var hookErr error
	var payload webhookUpdate
	err := json.NewDecoder(request.Body).Decode(&payload)
	if err == nil {
		if strings.EqualFold(payload.Action, "delete") {
			if trace.IsLogging(logger.DEBUG) {
				trace.Debug("In handleObjects. Delete webhook %s\n", objectType)
			}
			hookErr = deleteWebhook(orgID, objectType, payload.URL)
		} else if strings.EqualFold(payload.Action, "register") {
			if trace.IsLogging(logger.DEBUG) {
				trace.Debug("In handleObjects. Register webhook %s\n", objectType)
			}
			hookErr = registerWebhook(orgID, objectType, payload.URL)
		}
		if hookErr == nil {
			writer.WriteHeader(http.StatusOK)
		} else {
			communications.SendErrorResponse(writer, hookErr, "", 0)
		}
	} else {
		communications.SendErrorResponse(writer, err, "Invalid JSON for update. Error: ", http.StatusBadRequest)
	}
}

// swagger:operation PUT /api/v1/objects/{orgID}/{objectType}/{objectID} handleUpdateObject
//
// Update/create an object.
//
// Update/create the object of the specified object type and object ID.
// If an object with the same type and ID exists that object is updated, otherwise a new object is created.
//
// ---
//
// produces:
// - text/plain
//
// parameters:
// - name: orgID
//   in: path
//   description: The orgID of the object to update/create. Present only when working with a CSS, removed from the path when working with an ESS
//   required: true
//   type: string
// - name: objectType
//   in: path
//   description: The object type of the object to update/create
//   required: true
//   type: string
// - name: objectID
//   in: path
//   description: The object ID of the object to update/create
//   required: true
//   type: string
// - name: payload
//   in: body
//   required: true
//   schema:
//     "$ref": "#/definitions/objectUpdate"
//
// responses:
//   '200':
//     description: Object updated
//     schema:
//       type: string
//   '500':
//     description: Failed to update/create the object
//     schema:
//       type: string
func handleUpdateObject(orgID string, objectType string, objectID string, writer http.ResponseWriter, request *http.Request) {
	if trace.IsLogging(logger.DEBUG) {
		trace.Debug("In handleObjects. Update %s %s %s\n", orgID, objectType, objectID)
	}

	var payload objectUpdate
	err := json.NewDecoder(request.Body).Decode(&payload)
	if err == nil {
		username, password, ok := request.BasicAuth()
		if !ok || !security.CanUserCreateObject(username, password, orgID, &payload.Meta) {
			writer.WriteHeader(http.StatusForbidden)
			writer.Write(unauthorizedBytes)
			return
		}
		if err := updateObject(orgID, objectType, objectID, payload.Meta, payload.Data); err == nil {
			writer.WriteHeader(http.StatusOK)
		} else {
			communications.SendErrorResponse(writer, err, "", 0)
		}
	} else {
		communications.SendErrorResponse(writer, err, "Invalid JSON for update. Error: ", http.StatusBadRequest)
	}
}

// swagger:operation GET /api/v1/organizations handleGetOrganizations
//
// Get organizations.
//
// Get the list of existing organizations. Relevant to CSS only.
//
// ---
//
// produces:
// - application/json
// - text/plain
//
// parameters:
//
// responses:
//   '200':
//     description: Organizations response
//     schema:
//       type: array
//       items:
//         "$ref": "#/definitions/organization"
//   '404':
//     description: No organizations found
//     schema:
//       type: string
//   '500':
//     description: Failed to retrieve the organizations
//     schema:
//       type: string
func handleGetOrganizations(writer http.ResponseWriter, request *http.Request) {
	if !common.Running {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	username, password, ok := request.BasicAuth()
	if !ok {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}
	code, userOrg, _ := security.Authenticate(username, password)
	if code != security.AuthAdmin && code != security.AuthSyncAdmin {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}

	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
	if trace.IsLogging(logger.DEBUG) {
		trace.Debug("In handleGetOrganizations. Get the list of organizations.\n")
	}
	if orgs, err := getOrganizations(); err != nil {
		communications.SendErrorResponse(writer, err, "Failed to fetch the list of organizations. Error: ", 0)
	} else {
		if len(orgs) == 0 {
			writer.WriteHeader(http.StatusNotFound)
		} else {
			orgsList := make([]organization, 0)
			for _, org := range orgs {
				if code == security.AuthSyncAdmin || userOrg == org.OrgID {
					orgsList = append(orgsList, organization{OrgID: org.OrgID, Address: org.Address})
				}
			}
			if data, err := json.MarshalIndent(orgsList, "", "  "); err != nil {
				communications.SendErrorResponse(writer, err, "Failed to marshal the list of organizations. Error: ", 0)
			} else {
				writer.Header().Add(contentType, applicationJSON)
				writer.WriteHeader(http.StatusOK)
				writer.Write(data)
			}
		}
	}
}

func handleOrganizations(writer http.ResponseWriter, request *http.Request) {
	if !common.Running {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	var orgID string
	if len(request.URL.Path) == 0 {
		handleGetOrganizations(writer, request)
		return
	}

	parts := strings.Split(request.URL.Path, "/")
	if len(parts) != 1 && !(len(parts) == 2 && len(parts[1]) == 0) {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	orgID = parts[0]

	username, password, ok := request.BasicAuth()
	if !ok {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}
	code, userOrg, _ := security.Authenticate(username, password)
	if !((code == security.AuthAdmin && orgID == userOrg) || code == security.AuthSyncAdmin) {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}

	switch request.Method {
	// swagger:operation DELETE /api/v1/organizations/{orgID} handleOrganizations
	//
	// Delete organization.
	//
	// Remove organization information and clean up all resources (objects, destinations, etc.) all resources (objects, destinations, etc.) associated
	// with the deleted organization.
	//
	// ---
	//
	// produces:
	// - text/plain
	//
	// parameters:
	// - name: orgID
	//   in: path
	//   description: The orgID of the organization to delete.
	//   required: true
	//   type: string
	//
	// responses:
	//   '204':
	//     description: The organization was successfuly deleted
	//     schema:
	//       type: string
	//   '500':
	//     description: Failed to delete the organization
	//     schema:
	//       type: string
	case http.MethodDelete:
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("Deleting organization %s\n", orgID)
		}
		if err := deleteOrganization(orgID); err != nil {
			communications.SendErrorResponse(writer, err, "", 0)
		} else {
			writer.WriteHeader(http.StatusNoContent)
		}

	// swagger:operation PUT /api/v1/organizations/{orgID} handleOrganizations
	//
	// Update organization.
	//
	// Store organization information.
	//
	// ---
	//
	// produces:
	// - text/plain
	//
	// parameters:
	// - name: orgID
	//   in: path
	//   description: The orgID of the organization to update.
	//   required: true
	//   type: string
	// - name: payload
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/Organization"
	//
	// responses:
	//   '204':
	//     description: The organization was successfuly updated
	//     schema:
	//       type: string
	//   '500':
	//     description: Failed to update the organization
	//     schema:
	//       type: string
	case http.MethodPut:
		if trace.IsLogging(logger.DEBUG) {
			trace.Debug("Updating organization %s\n", orgID)
		}
		var payload common.Organization
		err := json.NewDecoder(request.Body).Decode(&payload)
		if err == nil {
			if err := updateOrganization(orgID, payload); err != nil {
				communications.SendErrorResponse(writer, err, "", 0)
			} else {
				writer.WriteHeader(http.StatusNoContent)
			}
		} else {
			communications.SendErrorResponse(writer, err, "Invalid JSON for update. Error: ", http.StatusBadRequest)
		}

	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleSecurity(writer http.ResponseWriter, request *http.Request) {
	if !common.Running {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	username, password, ok := request.BasicAuth()
	if !ok {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}
	code, userOrg, _ := security.Authenticate(username, password)
	if code == security.AuthFailed || code != security.AuthAdmin {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}

	parts := strings.Split(request.URL.Path, "/")
	if len(parts) < 2 || len(parts) > 4 {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	aclType := parts[0]
	orgID := parts[1]
	parts = parts[2:]

	if userOrg != orgID {
		writer.WriteHeader(http.StatusForbidden)
		writer.Write(unauthorizedBytes)
		return
	}

	if aclType != common.DestinationsACLType && aclType != common.ObjectsACLType {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	switch request.Method {
	case http.MethodDelete:
		if len(parts) != 2 {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		handleACLDelete(aclType, orgID, parts, writer)

	case http.MethodGet:
		if len(parts) > 1 {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		handleACLGet(aclType, orgID, parts, writer)

	case http.MethodPut:
		if len(parts) == 0 {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		handleACLUpdate(request, aclType, orgID, parts, writer)

	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// swagger:operation DELETE /api/v1/security/{type}/{orgID}/{key}/{username} handleACLDelete
//
// Remove a username from an ACL for a destination type or an object type.
//
// Remove a username from an ACL for a destination type or an object type. If the last username is removed,
// the ACL is deleted as well.
//
// ---
//
// produces:
// - text/plain
//
// parameters:
// - name: type
//   in: path
//   description: The type of the ACL to remove the specified username from.
//   required: true
//   type: string
//   enum: [destinations, objects]
// - name: orgID
//   in: path
//   description: The orgID in which the ACL for the destination type or object type exists.
//   required: true
//   type: string
// - name: key
//   in: path
//   description: The destination type or object type that is being protected by the ACL.
//   required: true
//   type: string
// - name: username
//   in: path
//   description: The username to remove from the specified ACL.
//   required: true
//   type: string
//
// responses:
//   '204':
//     description: The username was removed from the specified ACL.
//     schema:
//       type: string
//   '500':
//     description: Failed to remove the username from the specified ACL.
//     schema:
//       type: string
func handleACLDelete(aclType string, orgID string, parts []string, writer http.ResponseWriter) {
	usernames := append(make([]string, 0), parts[1])
	if err := removeUsersFromACL(aclType, orgID, parts[0], usernames); err == nil {
		writer.WriteHeader(http.StatusNoContent)
	} else {
		communications.SendErrorResponse(writer, err, "", 0)
	}
}

func handleACLGet(aclType string, orgID string, parts []string, writer http.ResponseWriter) {
	var results []string
	var err error
	var requestType string

	if len(parts) == 1 {
		// Get a single ACL

		// swagger:operation GET /api/v1/security/{type}/{orgID}/{key} handleACLGet
		//
		// Retrieve the list of usernames from an ACL for a destination type or an object type.
		//
		// ---
		//
		// produces:
		// - text/plain
		//
		// parameters:
		// - name: type
		//   in: path
		//   description: The type of the ACL whose username list should be retrieved.
		//   required: true
		//   type: string
		//   enum: [destinations, objects]
		// - name: orgID
		//   in: path
		//   description: The orgID in which the ACL for the destination type or object type exists.
		//   required: true
		//   type: string
		// - name: key
		//   in: path
		//   description: The destination type or object type that is being protected by the ACL.
		//   required: true
		//   type: string
		//
		// responses:
		//   '200':
		//     description: The list of usernames was retrieved from the specified ACL.
		//     schema:
		//       type: array
		//       items:
		//         type: string
		//   '404':
		//     description: ACL not found
		//     schema:
		//       type: string
		//   '500':
		//     description: Failed to retrieve the usernames from the specified ACL.
		//     schema:
		//       type: string

		results, err = retrieveACL(aclType, orgID, parts[0])
		requestType = "usernames"
	} else {
		// Get all ACLs

		// swagger:operation GET /api/v1/security/{type}/{orgID} handleACLGetAll
		//
		// Retrieve the list of destination or object ACLs for an organization.
		//
		// ---
		//
		// produces:
		// - text/plain
		//
		// parameters:
		// - name: type
		//   in: path
		//   description: The type of the ACL whose username list should be retrieved.
		//   required: true
		//   type: string
		//   enum: [destinations, objects]
		// - name: orgID
		//   in: path
		//   description: The orgID in which the ACL for the destination type or object type exists.
		//   required: true
		//   type: string
		//
		// responses:
		//   '200':
		//     description: The list of ACLs retrieved of the specified type.
		//     schema:
		//       type: array
		//       items:
		//         type: string
		//   '404':
		//     description: No ACLs found
		//     schema:
		//       type: string
		//   '500':
		//     description: Failed to retrieve the list of ACLs retrieved of the specified type.
		//     schema:
		//       type: string

		requestType = "ACLs"
		results, err = retrieveACLsInOrg(aclType, orgID)
	}

	if err != nil {
		communications.SendErrorResponse(writer, err, "", 0)
		return
	}

	if len(results) == 0 {
		writer.WriteHeader(http.StatusNotFound)
	} else {
		if data, err := json.MarshalIndent(results, "", "  "); err != nil {
			message := fmt.Sprintf("Failed to marshal the list of %s. Error: ", requestType)
			communications.SendErrorResponse(writer, err, message, 0)
		} else {
			writer.Header().Add(contentType, applicationJSON)
			writer.WriteHeader(http.StatusOK)
			writer.Write(data)
		}
	}
}

func handleACLUpdate(request *http.Request, aclType string, orgID string, parts []string, writer http.ResponseWriter) {
	if len(parts) == 2 {
		// swagger:operation PUT /api/v1/security/{type}/{orgID}/{key}/{username} handleACLUpdate
		//
		// Add a username to an ACL for a destination type or an object type.
		//
		// Add a username to an ACL for a destination type or an object type. If the first username is being added,
		// the ACL is created.
		//
		// ---
		//
		// produces:
		// - text/plain
		//
		// parameters:
		// - name: type
		//   in: path
		//   description: The type of the ACL to which the specified username will be added.
		//   required: true
		//   type: string
		//   enum: [destinations, objects]
		// - name: orgID
		//   in: path
		//   description: The orgID in which the ACL for the destination type or object type exists.
		//   required: true
		//   type: string
		// - name: key
		//   in: path
		//   description: The destination type or object type that is being protected by the ACL.
		//   required: true
		//   type: string
		// - name: username
		//   in: path
		//   description: The username to add to the specified ACL.
		//   required: true
		//   type: string
		//
		// responses:
		//   '204':
		//     description: The username was added to the specified ACL.
		//     schema:
		//       type: string
		//   '500':
		//     description: Failed to add the username to the specified ACL.
		//     schema:
		//       type: string
		usernames := append(make([]string, 0), parts[1])
		if err := addUsersToACL(aclType, orgID, parts[0], usernames); err == nil {
			writer.WriteHeader(http.StatusNoContent)
		} else {
			communications.SendErrorResponse(writer, err, "", 0)
		}
	} else {
		// Bulk add or bulk delete

		// swagger:operation PUT /api/v1/security/{type}/{orgID}/{key} handleBulkACLUpdate
		//
		// Bulk add/remove of username(s) to/from an ACL for a destination type or an object type.
		//
		// Bulk add/remove of username(s) to/from an ACL for a destination type or an object type. If the
		// first username is being added, the ACL is created. If the last username is removed, the ACL
		// is deleted.
		//
		// ---
		//
		// produces:
		// - text/plain
		//
		// parameters:
		// - name: type
		//   in: path
		//   description: The type of the ACL to which the specified username(s) will be added/removed.
		//   required: true
		//   type: string
		//   enum: [destinations, objects]
		// - name: orgID
		//   in: path
		//   description: The orgID in which the ACL for the destination type or object type exists.
		//   required: true
		//   type: string
		// - name: key
		//   in: path
		//   description: The destination type or object type that is being protected by the ACL.
		//   required: true
		//   type: string
		// - name: payload
		//   in: body
		//   required: true
		//   schema:
		//     "$ref": "#/definitions/bulkACLUpdate"
		//
		// responses:
		//   '204':
		//     description: The username(s) were added/removed to/from the specified ACL.
		//     schema:
		//       type: string
		//   '500':
		//     description: Failed to add/remove the username(s) to/from the specified ACL.
		//     schema:
		//       type: string
		var payload bulkACLUpdate
		err := json.NewDecoder(request.Body).Decode(&payload)
		if err == nil {

			var updateErr error
			if strings.EqualFold(payload.Action, "remove") {
				if trace.IsLogging(logger.DEBUG) {
					trace.Debug("In handleSecurity. Bulk remove usernames %s\n", parts[0])
				}
				updateErr = removeUsersFromACL(aclType, orgID, parts[0], payload.Usernames)
			} else if strings.EqualFold(payload.Action, "add") {
				if trace.IsLogging(logger.DEBUG) {
					trace.Debug("In handleSecurity. Bulk add usernames %s\n", parts[0])
				}
				updateErr = addUsersToACL(aclType, orgID, parts[0], payload.Usernames)
			} else {
				communications.SendErrorResponse(writer, nil, fmt.Sprintf("Invalid action (%s) in payload.", payload.Action), http.StatusBadRequest)
			}
			if updateErr == nil {
				writer.WriteHeader(http.StatusNoContent)
			} else {
				communications.SendErrorResponse(writer, updateErr, "", 0)
			}
		} else {
			communications.SendErrorResponse(writer, err, "Invalid JSON for update. Error: ", http.StatusBadRequest)
		}
	}
}

func canUserAccessObject(request *http.Request, orgID, objectType string) bool {
	username, password, ok := request.BasicAuth()
	return ok && security.CanUserAccessObject(username, password, orgID, objectType)
}