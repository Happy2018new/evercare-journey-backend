package trip

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func HandleCreateTrip(c *gin.Context) {
	const source = "HandleCreateTrip"
	var request CreateTripRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, CreateTripResponse{}, source, define.NewGeneralError(source, err, define.LangKeyTripRequestBodyInvalid))
		return
	}
	user, generalErr := loadTripUser(request.BasicSessionInfo, source)
	if generalErr != nil {
		respondTripError(c, CreateTripResponse{}, source, generalErr)
		return
	}
	startID, generalErr := validateAmapPlaceID(source, "start_amap_place_id", request.StartAmapPlaceID)
	if generalErr != nil {
		respondTripError(c, CreateTripResponse{}, source, generalErr)
		return
	}
	endID, generalErr := validateAmapPlaceID(source, "end_amap_place_id", request.EndAmapPlaceID)
	if generalErr != nil {
		respondTripError(c, CreateTripResponse{}, source, generalErr)
		return
	}
	if startID == endID {
		respondTripError(c, CreateTripResponse{}, source, invalidTripRequestWithKey(source, define.LangKeyTripStartEndSameInvalid, "start and end places must be different"))
		return
	}
	tripName, generalErr := normalizeTripName(source, request.TripName)
	if generalErr != nil {
		respondTripError(c, CreateTripResponse{}, source, generalErr)
		return
	}
	if generalErr = validateTripDate(source, request.TripDate); generalErr != nil {
		respondTripError(c, CreateTripResponse{}, source, generalErr)
		return
	}
	if generalErr = validateTravelMode(source, request.TravelMode); generalErr != nil {
		respondTripError(c, CreateTripResponse{}, source, generalErr)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	var created define.TripInfo
	transactionErr := environment.DB.Database().Transaction(func(tx *gorm.DB) error {
		startPlace, err := ensurePlaceByAmapID(ctx, tx, startID)
		if err != nil {
			return err
		}
		endPlace, err := ensurePlaceByAmapID(ctx, tx, endID)
		if err != nil {
			return err
		}
		created, err = environment.DB.TripHandle().CreateTripWithInfo(
			tx,
			user.UserUniqueID,
			tripName,
			request.TripDate,
			request.TravelMode,
			define.MulTripNode{
				{PlaceIdentity: startPlace.PlaceIdentity},
				{PlaceIdentity: endPlace.PlaceIdentity},
			},
		)
		return err
	})
	if transactionErr != nil {
		if created.TripIdentity != "" {
			// The SQL transaction and bbolt are separate stores. If the outer
			// transaction fails after the handle wrote nodes, remove that resource
			// so a later trip cannot observe an orphaned node list.
			_ = environment.DB.TripHandle().DeleteTripNodes(created.TripIdentity)
		}
		if typed, ok := transactionErr.(*define.GeneralError); ok {
			generalErr = typed
		} else {
			generalErr = define.NewGeneralError(source, transactionErr, define.LangKeyTripCreateUnknownErr)
		}
		respondTripError(c, CreateTripResponse{}, source, generalErr)
		return
	}
	c.JSON(http.StatusOK, CreateTripResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		TripIdentity:      created.TripIdentity,
		CurrentVersion:    created.CurrentVersion,
	})
}

func HandleQueryTrips(c *gin.Context) {
	const source = "HandleQueryTrips"
	var request QueryTripsRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, QueryTripsResponse{}, source, define.NewGeneralError(source, err, define.LangKeyTripRequestBodyInvalid))
		return
	}
	user, generalErr := loadTripUser(request.BasicSessionInfo, source)
	if generalErr != nil {
		respondTripError(c, QueryTripsResponse{}, source, generalErr)
		return
	}
	if generalErr = validateTripIdentityList(source, request.TripIdentity); generalErr != nil {
		respondTripError(c, QueryTripsResponse{}, source, generalErr)
		return
	}

	tripData := make([]TripData, 0, len(request.TripIdentity))
	for index, identity := range request.TripIdentity {
		tripInfo, generalErr := loadOwnedTrip(environment.DB.Database(), user.UserUniqueID, strings.TrimSpace(identity), source)
		if generalErr != nil {
			respondTripError(c, QueryTripsResponse{}, source, generalErr.AppendSource(fmt.Sprintf("index=%d", index)))
			return
		}
		nodes, foundNodes, nodesErr := environment.DB.TripHandle().LoadTripNodesWithError(tripInfo.TripIdentity)
		if nodesErr != nil {
			respondTripError(c, QueryTripsResponse{}, source, nodesErr)
			return
		}
		if !foundNodes {
			respondTripError(c, QueryTripsResponse{}, source, define.NewGeneralError(source, fmt.Errorf("trip %s has no node resource", tripInfo.TripIdentity), define.LangKeyTripDataCorrupt))
			return
		}
		if nodesErr = validateStoredTripNodes(source, nodes); nodesErr != nil {
			respondTripError(c, QueryTripsResponse{}, source, nodesErr)
			return
		}
		tripData = append(tripData, tripDataFromInfo(tripInfo, nodes))
	}
	c.JSON(http.StatusOK, QueryTripsResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		TripData:          tripData,
	})
}

func HandleQueryOwnedTrips(c *gin.Context) {
	const source = "HandleQueryOwnedTrips"
	var request QueryOwnedTripRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, QueryOwnedTripResponse{}, source, define.NewGeneralError(source, err, define.LangKeyTripRequestBodyInvalid))
		return
	}
	user, generalErr := loadTripUser(request.BasicSessionInfo, source)
	if generalErr != nil {
		respondTripError(c, QueryOwnedTripResponse{}, source, generalErr)
		return
	}
	trips, generalErr := environment.DB.TripHandle().QueryTripByOwnerWithError(environment.DB.Database(), user.UserUniqueID)
	if generalErr != nil {
		respondTripError(c, QueryOwnedTripResponse{}, source, generalErr)
		return
	}
	identities := make([]string, 0, len(trips))
	for _, tripInfo := range trips {
		identities = append(identities, tripInfo.TripIdentity)
	}
	c.JSON(http.StatusOK, QueryOwnedTripResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		TripIdentity:      identities,
	})
}

func HandleQueryTripVersion(c *gin.Context) {
	const source = "HandleQueryTripVersion"
	var request QueryTripVersionRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, QueryTripVersionResponse{}, source, define.NewGeneralError(source, err, define.LangKeyTripRequestBodyInvalid))
		return
	}
	user, generalErr := loadTripUser(request.BasicSessionInfo, source)
	if generalErr != nil {
		respondTripError(c, QueryTripVersionResponse{}, source, generalErr)
		return
	}
	if generalErr = validateTripIdentityList(source, request.TripIdentity); generalErr != nil {
		respondTripError(c, QueryTripVersionResponse{}, source, generalErr)
		return
	}
	versions := make([]TripVersionData, 0, len(request.TripIdentity))
	for index, identity := range request.TripIdentity {
		tripInfo, generalErr := loadOwnedTrip(environment.DB.Database(), user.UserUniqueID, strings.TrimSpace(identity), source)
		if generalErr != nil {
			respondTripError(c, QueryTripVersionResponse{}, source, generalErr.AppendSource(fmt.Sprintf("index=%d", index)))
			return
		}
		versions = append(versions, TripVersionData{
			TripIdentity: tripInfo.TripIdentity,
			Version:      tripInfo.CurrentVersion,
		})
	}
	c.JSON(http.StatusOK, QueryTripVersionResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		TripVersion:       versions,
	})
}

func HandleUpdateTrip(c *gin.Context) {
	const source = "HandleUpdateTrip"
	var request UpdateTripRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, UpdateTripResponse{}, source, define.NewGeneralError(source, err, define.LangKeyTripRequestBodyInvalid))
		return
	}
	user, generalErr := loadTripUser(request.BasicSessionInfo, source)
	if generalErr != nil {
		respondTripError(c, UpdateTripResponse{}, source, generalErr)
		return
	}
	tripIdentity, generalErr := canonicalUUIDIdentity(source, "trip_identity", request.TripIdentity)
	if generalErr != nil {
		respondTripError(c, UpdateTripResponse{}, source, generalErr)
		return
	}
	tripName, generalErr := normalizeTripName(source, request.TripName)
	if generalErr != nil {
		respondTripError(c, UpdateTripResponse{}, source, generalErr)
		return
	}
	if generalErr = validateTripDate(source, request.TripDate); generalErr != nil {
		respondTripError(c, UpdateTripResponse{}, source, generalErr)
		return
	}
	if generalErr = validateTravelMode(source, request.TravelMode); generalErr != nil {
		respondTripError(c, UpdateTripResponse{}, source, generalErr)
		return
	}
	if generalErr = validateTripStatus(source, request.TripStatus); generalErr != nil {
		respondTripError(c, UpdateTripResponse{}, source, generalErr)
		return
	}

	var updatedVersion uint32
	generalErr = environment.DB.TripHandle().UpdateTrip(
		environment.DB.Database(),
		tripIdentity,
		func(tx *gorm.DB, tripInfo *define.TripInfo) *define.GeneralError {
			if tripInfo.UserUniqueID != user.UserUniqueID {
				return define.NewGeneralError("HandleUpdateTrip", fmt.Errorf("target trip not found"), define.LangKeyTripQueryNotFoundErr)
			}
			if request.ExpectedVersion != nil && *request.ExpectedVersion != tripInfo.CurrentVersion {
				return invalidTripRequestWithKey("HandleUpdateTrip", define.LangKeyTripVersionConflict, "expected trip version %d but current version is %d", *request.ExpectedVersion, tripInfo.CurrentVersion)
			}
			if editableErr := validateTripEditable("HandleUpdateTrip", tripInfo.TripStatus); editableErr != nil {
				return editableErr
			}
			if transitionErr := validateTripStatusTransition("HandleUpdateTrip", tripInfo.TripStatus, request.TripStatus); transitionErr != nil {
				return transitionErr
			}
			var versionErr *define.GeneralError
			updatedVersion, versionErr = nextTripVersion("HandleUpdateTrip", tripInfo.CurrentVersion)
			if versionErr != nil {
				return versionErr
			}
			tripInfo.TripName = tripName
			tripInfo.TripDate = request.TripDate
			tripInfo.TravelMode = request.TravelMode
			tripInfo.TripStatus = request.TripStatus
			return nil
		},
	)
	if generalErr != nil {
		respondTripError(c, UpdateTripResponse{}, source, generalErr)
		return
	}
	c.JSON(http.StatusOK, UpdateTripResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		TripVersion:       updatedVersion,
	})
}

func HandleDeleteTrip(c *gin.Context) {
	const source = "HandleDeleteTrip"
	var request DeleteTripRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, DeleteTripResponse{}, source, define.NewGeneralError(source, err, define.LangKeyTripRequestBodyInvalid))
		return
	}
	user, generalErr := loadTripUser(request.BasicSessionInfo, source)
	if generalErr != nil {
		respondTripError(c, DeleteTripResponse{}, source, generalErr)
		return
	}
	tripIdentity, generalErr := canonicalUUIDIdentity(source, "trip_identity", request.TripIdentity)
	if generalErr != nil {
		respondTripError(c, DeleteTripResponse{}, source, generalErr)
		return
	}
	if _, generalErr = loadOwnedTrip(environment.DB.Database(), user.UserUniqueID, tripIdentity, source); generalErr != nil {
		respondTripError(c, DeleteTripResponse{}, source, generalErr)
		return
	}
	if generalErr = environment.DB.TripHandle().DeleteTrip(environment.DB.Database(), tripIdentity); generalErr != nil {
		respondTripError(c, DeleteTripResponse{}, source, generalErr)
		return
	}
	c.JSON(http.StatusOK, DeleteTripResponse{BasicResponseInfo: general.SuccResponseInfo()})
}

func HandleEditTripNode(c *gin.Context) {
	const source = "HandleEditTripNode"
	var request EditTripNodeRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, EditTripNodeResponse{}, source, define.NewGeneralError(source, err, define.LangKeyTripRequestBodyInvalid))
		return
	}
	user, generalErr := loadTripUser(request.BasicSessionInfo, source)
	if generalErr != nil {
		respondTripError(c, EditTripNodeResponse{}, source, generalErr)
		return
	}
	tripIdentity, generalErr := canonicalUUIDIdentity(source, "trip_identity", request.TripIdentity)
	if generalErr != nil {
		respondTripError(c, EditTripNodeResponse{}, source, generalErr)
		return
	}
	if request.RequestAction > EditTripNodeRequestActionUpdate {
		respondTripError(c, EditTripNodeResponse{}, source, invalidTripRequestWithKey(source, define.LangKeyTripNodeActionInvalid, "unsupported request_action %d", request.RequestAction))
		return
	}
	if request.RequestAction == EditTripNodeRequestActionAdd || request.RequestAction == EditTripNodeRequestActionUpdate {
		if _, generalErr = validateAmapPlaceID(source, "amap_place_id", request.AmapPlaceID); generalErr != nil {
			respondTripError(c, EditTripNodeResponse{}, source, generalErr)
			return
		}
	}
	noteString := uuid.Nil
	if request.RequestAction == EditTripNodeRequestActionAdd || request.RequestAction == EditTripNodeRequestActionUpdate {
		noteString, generalErr = parseTripNodeNote(source, request.NoteString)
		if generalErr != nil {
			respondTripError(c, EditTripNodeResponse{}, source, generalErr)
			return
		}
	}
	if request.RequestAction == EditTripNodeRequestActionMove && request.NodeIndex == request.MoveToInd {
		tripInfo, checkErr := loadOwnedTrip(environment.DB.Database(), user.UserUniqueID, tripIdentity, source)
		if checkErr != nil {
			respondTripError(c, EditTripNodeResponse{}, source, checkErr)
			return
		}
		if checkErr = validateTripEditable(source, tripInfo.TripStatus); checkErr != nil {
			respondTripError(c, EditTripNodeResponse{}, source, checkErr)
			return
		}
		if request.ExpectedVersion != nil && *request.ExpectedVersion != tripInfo.CurrentVersion {
			respondTripError(c, EditTripNodeResponse{}, source, invalidTripRequestWithKey(source, define.LangKeyTripVersionConflict, "expected trip version %d but current version is %d", *request.ExpectedVersion, tripInfo.CurrentVersion))
			return
		}
		nodes, foundNodes, nodesErr := environment.DB.TripHandle().LoadTripNodesWithError(tripInfo.TripIdentity)
		if nodesErr != nil {
			respondTripError(c, EditTripNodeResponse{}, source, nodesErr)
			return
		}
		if !foundNodes {
			nodesErr = define.NewGeneralError(source, fmt.Errorf("trip has no node resource"), define.LangKeyTripDataCorrupt)
			respondTripError(c, EditTripNodeResponse{}, source, nodesErr)
			return
		}
		if nodesErr = validateStoredTripNodes(source, nodes); nodesErr != nil {
			respondTripError(c, EditTripNodeResponse{}, source, nodesErr)
			return
		}
		if int(request.NodeIndex) >= len(nodes) || int(request.MoveToInd) >= len(nodes) {
			respondTripError(c, EditTripNodeResponse{}, source, invalidTripRequestWithKey(source, define.LangKeyTripNodeIndexInvalid, "node_index and move_to_ind must refer to existing nodes"))
			return
		}
		if request.NodeIndex == 0 || int(request.NodeIndex) >= len(nodes)-1 {
			respondTripError(c, EditTripNodeResponse{}, source, invalidTripRequestWithKey(source, define.LangKeyTripNodeEndpointProtected, "only intermediate nodes can be moved"))
			return
		}
		c.JSON(http.StatusOK, EditTripNodeResponse{BasicResponseInfo: general.SuccResponseInfo(), TripVersion: tripInfo.CurrentVersion})
		return
	}

	var updatedVersion uint32
	var previousNodes define.MulTripNode
	var savedTripIdentity string
	nodesSaved := false
	generalErr = environment.DB.TripHandle().UpdateTrip(
		environment.DB.Database(),
		tripIdentity,
		func(tx *gorm.DB, tripInfo *define.TripInfo) *define.GeneralError {
			if tripInfo.UserUniqueID != user.UserUniqueID {
				return define.NewGeneralError(source, fmt.Errorf("target trip not found"), define.LangKeyTripQueryNotFoundErr)
			}
			if request.ExpectedVersion != nil && *request.ExpectedVersion != tripInfo.CurrentVersion {
				return invalidTripRequestWithKey(source, define.LangKeyTripVersionConflict, "expected trip version %d but current version is %d", *request.ExpectedVersion, tripInfo.CurrentVersion)
			}
			if editableErr := validateTripEditable(source, tripInfo.TripStatus); editableErr != nil {
				return editableErr
			}
			savedTripIdentity = tripInfo.TripIdentity
			nodes, foundNodes, nodesErr := environment.DB.TripHandle().LoadTripNodesWithError(tripInfo.TripIdentity)
			if nodesErr != nil {
				return nodesErr
			}
			if !foundNodes {
				return define.NewGeneralError(source, fmt.Errorf("trip has no node resource"), define.LangKeyTripDataCorrupt)
			}
			if nodesErr = validateStoredTripNodes(source, nodes); nodesErr != nil {
				return nodesErr
			}
			previousNodes = append(define.MulTripNode(nil), nodes...)
			// Calculate the version before touching bbolt. If the version is
			// exhausted, returning after SaveTripNodes would require an external
			// restore and could overwrite a concurrent node edit.
			var versionErr *define.GeneralError
			updatedVersion, versionErr = nextTripVersion(source, tripInfo.CurrentVersion)
			if versionErr != nil {
				return versionErr
			}
			switch request.RequestAction {
			case EditTripNodeRequestActionAdd:
				if len(nodes) >= maxTripNodeCount {
					return invalidTripRequestWithKey(source, define.LangKeyTripNodeCountInvalid, "trip cannot contain more than %d nodes", maxTripNodeCount)
				}
				if request.NodeIndex == 0 || int(request.NodeIndex) >= len(nodes) {
					return invalidTripRequestWithKey(source, define.LangKeyTripNodeIndexInvalid, "node_index must insert before the final node")
				}
				place, err := ensurePlaceByAmapID(c.Request.Context(), tx, strings.TrimSpace(request.AmapPlaceID))
				if err != nil {
					return err
				}
				node := define.TripNode{PlaceIdentity: place.PlaceIdentity, NoteString: noteString}
				nodes = append(nodes, define.TripNode{})
				copy(nodes[request.NodeIndex+1:], nodes[request.NodeIndex:])
				nodes[request.NodeIndex] = node
			case EditTripNodeRequestActionDelete:
				if request.NodeIndex == 0 || int(request.NodeIndex) >= len(nodes)-1 {
					return invalidTripRequestWithKey(source, define.LangKeyTripNodeEndpointProtected, "start and end nodes cannot be deleted")
				}
				nodes = append(nodes[:request.NodeIndex], nodes[request.NodeIndex+1:]...)
			case EditTripNodeRequestActionMove:
				if int(request.NodeIndex) >= len(nodes) || int(request.MoveToInd) >= len(nodes) {
					return invalidTripRequestWithKey(source, define.LangKeyTripNodeIndexInvalid, "node_index and move_to_ind must refer to existing nodes")
				}
				if request.NodeIndex == 0 || int(request.NodeIndex) >= len(nodes)-1 || request.MoveToInd == 0 || int(request.MoveToInd) >= len(nodes)-1 {
					return invalidTripRequestWithKey(source, define.LangKeyTripNodeEndpointProtected, "only intermediate nodes can be moved")
				}
				if request.NodeIndex != request.MoveToInd {
					node := nodes[request.NodeIndex]
					nodes = append(nodes[:request.NodeIndex], nodes[request.NodeIndex+1:]...)
					nodes = append(nodes, define.TripNode{})
					copy(nodes[request.MoveToInd+1:], nodes[request.MoveToInd:])
					nodes[request.MoveToInd] = node
				}
			case EditTripNodeRequestActionUpdate:
				if request.NodeIndex == 0 || int(request.NodeIndex) >= len(nodes)-1 {
					return invalidTripRequestWithKey(source, define.LangKeyTripNodeEndpointProtected, "start and end nodes cannot be updated")
				}
				place, err := ensurePlaceByAmapID(c.Request.Context(), tx, strings.TrimSpace(request.AmapPlaceID))
				if err != nil {
					return err
				}
				nodes[request.NodeIndex] = define.TripNode{PlaceIdentity: place.PlaceIdentity, NoteString: noteString}
			}
			if err := environment.DB.TripHandle().SaveTripNodes(tripInfo.TripIdentity, nodes); err != nil {
				return err
			}
			nodesSaved = true
			return nil
		},
	)
	if generalErr != nil {
		if nodesSaved && savedTripIdentity != "" {
			if restoreErr := environment.DB.TripHandle().SaveTripNodes(savedTripIdentity, previousNodes); restoreErr != nil {
				generalErr = define.NewGeneralError(source, restoreErr, define.LangKeyTripDataCorrupt)
			}
		}
		respondTripError(c, EditTripNodeResponse{}, source, generalErr)
		return
	}
	c.JSON(http.StatusOK, EditTripNodeResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		TripVersion:       updatedVersion,
	})
}
