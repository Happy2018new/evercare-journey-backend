package trip

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/database/handle"
	"github.com/Happy2018new/evercare-journey-backend/environment"
	"github.com/Happy2018new/evercare-journey-backend/service/general"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type optimizationPoint struct {
	node  define.TripNode
	place define.PlaceInfo
}

type optimizationCost struct {
	distance int64
	duration int64
}

// HandleOptimizeTrip uses the route costs returned by Amap as the road-network
// abstraction available to this service. It keeps the user-selected endpoints
// fixed, orders intermediate places with a nearest-neighbour seed, and applies
// directed 2-opt improvement. Elevation, rest-point density, medical access,
// and offline tile graphs are not present in the current data/API contract and
// therefore cannot be scored here without inventing data.
func HandleOptimizeTrip(c *gin.Context) {
	const source = "HandleOptimizeTrip"
	var request OptimizeTripRequest
	if err := c.ShouldBind(&request); err != nil {
		respondTripError(c, OptimizeTripResponse{}, source, define.NewGeneralError(source, err, define.LangKeyTripRequestBodyInvalid))
		return
	}
	user, generalErr := loadTripUser(request.BasicSessionInfo, source)
	if generalErr != nil {
		respondTripError(c, OptimizeTripResponse{}, source, generalErr)
		return
	}
	tripIdentity, generalErr := canonicalUUIDIdentity(source, "trip_identity", request.TripIdentity)
	if generalErr != nil {
		respondTripError(c, OptimizeTripResponse{}, source, generalErr)
		return
	}

	tripInfo, generalErr := loadOwnedTrip(environment.DB.Database(), user.UserUniqueID, tripIdentity, source)
	if generalErr != nil {
		respondTripError(c, OptimizeTripResponse{}, source, generalErr)
		return
	}
	if generalErr = validateTravelMode(source, tripInfo.TravelMode); generalErr != nil {
		respondTripError(c, OptimizeTripResponse{}, source, generalErr)
		return
	}
	if tripInfo.TripStatus == define.TripStatusCompleted || tripInfo.TripStatus == define.TripStatusCancelled {
		respondTripError(c, OptimizeTripResponse{}, source, define.NewGeneralError(
			source,
			fmt.Errorf("trip with terminal status %d cannot be optimized", tripInfo.TripStatus),
			define.LangKeyTripOptimizeStatusInvalid,
		))
		return
	}
	if tripInfo.TravelMode == define.TripTravelModeTransit {
		respondTripError(c, OptimizeTripResponse{}, source, define.NewGeneralError(
			source,
			fmt.Errorf("transit optimization is not available until a transit route API is configured"),
			define.LangKeyTripOptimizeTransitUnsupported,
		))
		return
	}
	nodes, foundNodes, nodesErr := environment.DB.TripHandle().LoadTripNodesWithError(tripInfo.TripIdentity)
	if nodesErr != nil {
		respondTripError(c, OptimizeTripResponse{}, source, nodesErr)
		return
	}
	if !foundNodes {
		respondTripError(c, OptimizeTripResponse{}, source, define.NewGeneralError(source, fmt.Errorf("trip has no node resource"), define.LangKeyTripDataCorrupt))
		return
	}
	if nodesErr = validateStoredTripNodes(source, nodes); nodesErr != nil {
		respondTripError(c, OptimizeTripResponse{}, source, nodesErr)
		return
	}
	if len(nodes) < 2 || len(nodes) > maxOptimizationNodeCount {
		respondTripError(c, OptimizeTripResponse{}, source, define.NewGeneralError(source, fmt.Errorf("trip must contain between 2 and %d nodes for optimization", maxOptimizationNodeCount), define.LangKeyTripOptimizeNodeCountInvalid))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	optimizedNodes, generalErr := optimizeTripNodes(ctx, nodes, tripInfo.TravelMode)
	if generalErr != nil {
		respondTripError(c, OptimizeTripResponse{}, source, generalErr)
		return
	}

	var newTrip define.TripInfo
	transactionErr := environment.DB.Database().Transaction(func(tx *gorm.DB) error {
		// Hold the source row lock through the duplicate creation. This closes
		// the race where a source update lands between the version check and the
		// INSERT of the optimized copy.
		latestTrip, found, queryErr := environment.DB.TripHandle().QueryTripByIdentity(
			tx.Clauses(clause.Locking{Strength: "UPDATE"}),
			tripInfo.TripIdentity,
		)
		if queryErr != nil {
			return queryErr
		}
		if !found || latestTrip.UserUniqueID != user.UserUniqueID {
			return define.NewGeneralError(source, fmt.Errorf("source trip changed while optimization was running"), define.LangKeyTripOptimizeSourceChanged)
		}
		if latestTrip.CurrentVersion != tripInfo.CurrentVersion {
			return define.NewGeneralError(source, fmt.Errorf("source trip changed while optimization was running"), define.LangKeyTripOptimizeSourceChanged)
		}

		for attempt := 0; attempt <= 100; attempt++ {
			optimizedName, nameErr := findOptimizedTripName(tx, user.UserUniqueID, tripInfo.TripName)
			if nameErr != nil {
				return nameErr
			}
			newTrip, generalErr = environment.DB.TripHandle().CreateTripWithStatus(
				tx,
				user.UserUniqueID,
				optimizedName,
				tripInfo.TripDate,
				tripInfo.TravelMode,
				define.TripStatusInPlanning,
				optimizedNodes,
			)
			if generalErr == nil {
				return nil
			}
			if generalErr.GetPublicMsg().LangKey != define.LangKeyTripCreateNameUsedErr || attempt == 100 {
				return generalErr
			}
		}
		return define.NewGeneralError(source, fmt.Errorf("could not generate a unique optimized trip name"), define.LangKeyTripOptimizeUnknownErr)
	})
	if transactionErr != nil {
		// SQL and bbolt are separate stores. CreateTripWithStatus writes the
		// node resource before the surrounding transaction commits, so an outer
		// commit failure must remove the resource created for this copy.
		if newTrip.TripIdentity != "" {
			_ = environment.DB.TripHandle().DeleteTripNodes(newTrip.TripIdentity)
		}
		if typed, ok := transactionErr.(*define.GeneralError); ok {
			respondTripError(c, OptimizeTripResponse{}, source, typed)
		} else {
			respondTripError(c, OptimizeTripResponse{}, source, define.NewGeneralError(source, transactionErr, define.LangKeyTripOptimizeUnknownErr))
		}
		return
	}

	c.JSON(http.StatusOK, OptimizeTripResponse{
		BasicResponseInfo: general.SuccResponseInfo(),
		NewTripData:       tripDataFromInfo(newTrip, optimizedNodes),
	})
}

func optimizeTripNodes(ctx context.Context, nodes define.MulTripNode, travelMode uint8) (define.MulTripNode, *define.GeneralError) {
	const source = "optimizeTripNodes"
	if len(nodes) < 2 {
		return nil, define.NewGeneralError(source, fmt.Errorf("at least two nodes are required"), define.LangKeyTripOptimizeNodeCountInvalid)
	}
	if generalErr := validateTravelMode(source, travelMode); generalErr != nil {
		return nil, generalErr
	}

	points := make([]optimizationPoint, len(nodes))
	for index, node := range nodes {
		if generalErr := validateUUIDIdentity(source, fmt.Sprintf("nodes[%d].place_identity", index), node.PlaceIdentity); generalErr != nil {
			return nil, generalErr
		}
		place, found, generalErr := environment.DB.TripHandle().QueryPlace(
			ctx,
			environment.DB.Database(),
			handle.QueryPlaceActionSearchByIdentity,
			node.PlaceIdentity,
		)
		if generalErr != nil {
			return nil, generalErr.AppendSource(source)
		}
		if !found || place.PlaceStatus != define.PlaceStatusActive || !validPlaceCoordinate(place) {
			return nil, define.NewGeneralError(
				source,
				fmt.Errorf("node %d refers to an unavailable place", index),
				define.LangKeyTripOptimizePlaceInvalid,
			)
		}
		points[index] = optimizationPoint{node: node, place: place}
	}
	if len(points) == 2 {
		return append(define.MulTripNode(nil), nodes...), nil
	}

	matrix, generalErr := buildOptimizationMatrix(ctx, points, travelMode)
	if generalErr != nil {
		return nil, generalErr
	}
	order := makeOptimizedOrder(matrix)
	result := make(define.MulTripNode, 0, len(order))
	for _, index := range order {
		result = append(result, points[index].node)
	}
	return result, nil
}

func buildOptimizationMatrix(ctx context.Context, points []optimizationPoint, travelMode uint8) ([][]optimizationCost, *define.GeneralError) {
	const source = "buildOptimizationMatrix"
	matrix := make([][]optimizationCost, len(points))
	for index := range matrix {
		matrix[index] = make([]optimizationCost, len(points))
	}

	distanceType := utils.AmapDistanceTypeWalking
	switch travelMode {
	case define.TripTravelModeDriving:
		distanceType = utils.AmapDistanceTypeDriving
	case define.TripTravelModeTransit:
		return nil, define.NewGeneralError(source, fmt.Errorf("transit optimization is not supported"), define.LangKeyTripOptimizeTransitUnsupported)
	case define.TripTravelModeWalking:
		distanceType = utils.AmapDistanceTypeWalking
	default:
		return nil, invalidTripRequestWithKey(source, define.LangKeyTripTravelModeInvalid, "unsupported travel_mode %d", travelMode)
	}

	origins := make([]utils.AmapCoordinate, len(points))
	for originIndex, origin := range points {
		origins[originIndex] = utils.AmapCoordinate{
			Longitude: origin.place.Longitude,
			Latitude:  origin.place.Latitude,
		}
	}
	parallelLimit := 8
	if len(points) < parallelLimit {
		parallelLimit = len(points)
	}
	semaphore := make(chan struct{}, parallelLimit)
	errCh := make(chan error, len(points))
	var waitGroup sync.WaitGroup
	for destinationIndex, destination := range points {
		destinationIndex, destination := destinationIndex, destination
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-semaphore }()

			distances, err := utils.QueryAmapDistances(
				ctx,
				origins,
				utils.AmapCoordinate{
					Longitude: destination.place.Longitude,
					Latitude:  destination.place.Latitude,
				},
				distanceType,
			)
			if err != nil {
				errCh <- err
				return
			}
			seen := make([]bool, len(points))
			for _, distance := range distances {
				if distance.OriginIndex < 0 || distance.OriginIndex >= len(points) {
					errCh <- fmt.Errorf("distance API returned invalid origin index %d", distance.OriginIndex)
					return
				}
				if distance.Distance < 0 || distance.Duration < 0 {
					errCh <- fmt.Errorf("distance API returned negative cost for origin %d", distance.OriginIndex)
					return
				}
				if distance.OriginIndex != destinationIndex && (distance.Distance == 0 || distance.Duration == 0) {
					errCh <- fmt.Errorf("distance API returned an unavailable route from origin %d", distance.OriginIndex)
					return
				}
				if seen[distance.OriginIndex] {
					errCh <- fmt.Errorf("distance API returned duplicate origin %d", distance.OriginIndex)
					return
				}
				matrix[distance.OriginIndex][destinationIndex] = optimizationCost{
					distance: distance.Distance,
					duration: distance.Duration,
				}
				seen[distance.OriginIndex] = true
			}
			for originIndex, found := range seen {
				if originIndex == destinationIndex {
					matrix[originIndex][destinationIndex] = optimizationCost{}
					continue
				}
				if !found {
					errCh <- fmt.Errorf("distance API did not return origin %d", originIndex)
					return
				}
			}
		}()
	}
	waitGroup.Wait()
	select {
	case err := <-errCh:
		return nil, define.NewGeneralError(source, err, define.LangKeyTripOptimizeRouteUnavailable)
	default:
	}
	if err := ctx.Err(); err != nil {
		return nil, define.NewGeneralError(source, err, define.LangKeyTripOptimizeUnknownErr)
	}
	return matrix, nil
}

func optimizationScore(cost optimizationCost) float64 {
	// Travel duration is the primary low-load proxy; distance breaks ties and
	// discourages unnecessarily long paths when durations are equal.
	return float64(cost.duration) + float64(cost.distance)*0.2
}

func makeOptimizedOrder(matrix [][]optimizationCost) []int {
	count := len(matrix)
	if count <= 2 {
		return []int{0, count - 1}
	}
	remaining := make(map[int]struct{}, count-2)
	for index := 1; index < count-1; index++ {
		remaining[index] = struct{}{}
	}
	order := []int{0}
	current := 0
	for len(remaining) > 0 {
		bestIndex := -1
		bestScore := 0.0
		for candidate := range remaining {
			score := optimizationScore(matrix[current][candidate])
			if bestIndex < 0 || score < bestScore || (score == bestScore && candidate < bestIndex) {
				bestIndex = candidate
				bestScore = score
			}
		}
		order = append(order, bestIndex)
		delete(remaining, bestIndex)
		current = bestIndex
	}
	order = append(order, count-1)

	for changed := true; changed; {
		changed = false
		for left := 1; left < len(order)-2; left++ {
			for right := left + 1; right < len(order)-1; right++ {
				candidate := append([]int(nil), order...)
				for i, j := left, right; i < j; i, j = i+1, j-1 {
					candidate[i], candidate[j] = candidate[j], candidate[i]
				}
				if optimizedOrderScore(candidate, matrix)+0.000001 < optimizedOrderScore(order, matrix) {
					order = candidate
					changed = true
				}
			}
		}
	}
	return order
}

func optimizedOrderScore(order []int, matrix [][]optimizationCost) float64 {
	var score float64
	for index := 1; index < len(order); index++ {
		score += optimizationScore(matrix[order[index-1]][order[index]])
	}
	return score
}

func findOptimizedTripName(tx *gorm.DB, userUniqueID uint32, originalName string) (string, *define.GeneralError) {
	for index := 0; index <= 100; index++ {
		suffix := "-优化"
		if index > 0 {
			suffix = fmt.Sprintf("-优化-%d", index)
		}
		candidate := tripNameWithSuffix(originalName, suffix)
		_, found, generalErr := environment.DB.TripHandle().QueryTripByOwnerAndName(tx, userUniqueID, candidate)
		if generalErr != nil {
			return "", generalErr.AppendSource("findOptimizedTripName")
		}
		if !found {
			return candidate, nil
		}
	}
	return "", define.NewGeneralError("findOptimizedTripName", fmt.Errorf("could not generate a unique optimized trip name"), define.LangKeyTripOptimizeUnknownErr)
}

func truncateTripName(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > define.TripNameMaxLengthDefault {
		runes = runes[:define.TripNameMaxLengthDefault]
	}
	return string(runes)
}

func tripNameWithSuffix(originalName string, suffix string) string {
	suffixRunes := []rune(suffix)
	if len(suffixRunes) >= define.TripNameMaxLengthDefault {
		return truncateTripName(suffix)
	}
	originalRunes := []rune(strings.TrimSpace(originalName))
	maxOriginalLength := define.TripNameMaxLengthDefault - len(suffixRunes)
	if len(originalRunes) > maxOriginalLength {
		originalRunes = originalRunes[:maxOriginalLength]
	}
	name := string(originalRunes) + suffix
	if strings.TrimSpace(name) == suffix {
		name = "优化行程" + suffix
	}
	return truncateTripName(name)
}
