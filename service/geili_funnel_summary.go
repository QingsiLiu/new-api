package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const (
	funnelRawRetentionSeconds = int64(180 * 86400)
	funnelSmallCellMinimum    = int64(5)
)

var ErrInvalidFunnelSummaryQuery = errors.New("invalid funnel summary query")

type FunnelSummaryFacts struct {
	EntryVisitors            []model.FunnelVisitorFact
	LinkedFirstTouches       map[int]model.FunnelVisitorFact
	UserCreated              map[int]int64
	FirstTopUps              map[int]int64
	FirstTasks               map[int]int64
	StudioOpens              map[int][]int64
	IndependentRegistrations []model.FunnelTimedUser
	IndependentTopUps        []model.FunnelTimedUser
	IndependentTasks         []model.FunnelTimedUser
	IndependentStudio        []model.FunnelTimedUser
	ActivityDays             []model.FunnelActivityFact
	Failures                 []model.FunnelFailureFact
	TaskStatuses             []model.FunnelTaskStatusFact
	CollectionStartedAt      int64
	LastEventAt              int64
	DuplicateSinceStart      uint64
	RejectedSinceStart       uint64
	CounterSince             int64
	InvalidTopUpTimes        int64
	InvalidTaskTimes         int64
}

type FunnelSummaryQuery struct {
	Environment string
	Dimension   string
	From        int64
	To          int64
	Now         int64
}

type funnelSegmentFilter struct {
	dimension string
	value     string
}

func BuildFunnelSummary(query FunnelSummaryQuery, facts FunnelSummaryFacts) (dto.GeiliFunnelSummaryResponse, error) {
	if err := validateFunnelSummaryQuery(query); err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}

	rawCutoff := query.Now - funnelRawRetentionSeconds
	overall := buildFunnelMetrics(query, facts, nil, false, rawCutoff)
	response := dto.GeiliFunnelSummaryResponse{
		Window: dto.FunnelWindow{
			From:                time.Unix(query.From, 0).UTC().Format(time.RFC3339),
			To:                  time.Unix(query.To, 0).UTC().Format(time.RFC3339),
			Timezone:            "UTC",
			Environment:         query.Environment,
			CollectionStartedAt: facts.CollectionStartedAt,
			RawCutoff:           rawCutoff,
		},
		Metrics: overall,
		Quality: dto.FunnelQuality{
			DuplicateSinceStart: facts.DuplicateSinceStart,
			RejectedSinceStart:  facts.RejectedSinceStart,
			CounterSince:        facts.CounterSince,
			LastEventAt:         facts.LastEventAt,
			InvalidTopUpTimes:   facts.InvalidTopUpTimes,
			InvalidTaskTimes:    facts.InvalidTaskTimes,
		},
		Segments: make([]dto.FunnelSegment, 0),
	}
	for _, visitor := range facts.EntryVisitors {
		switch visitor.IdentityState {
		case model.FunnelIdentityAmbiguous:
			response.Quality.Ambiguous++
		case model.FunnelIdentityAnonymous:
			response.Quality.Unlinked++
		}
	}

	if query.Dimension == "all" {
		response.Segments = append(response.Segments, dto.FunnelSegment{
			Dimension: "all",
			Value:     "all",
			Metrics:   overall,
		})
		return response, nil
	}

	entryCounts := make(map[string]int64)
	for _, visitor := range facts.EntryVisitors {
		value := visitorDimension(visitor, query.Dimension)
		if value != "" {
			entryCounts[value]++
		}
	}
	values := make([]string, 0, len(entryCounts))
	for value, count := range entryCounts {
		if count < funnelSmallCellMinimum {
			response.Quality.SuppressedSegments++
			continue
		}
		values = append(values, value)
	}
	sort.Strings(values)
	for _, value := range values {
		filter := &funnelSegmentFilter{dimension: query.Dimension, value: value}
		response.Segments = append(response.Segments, dto.FunnelSegment{
			Dimension: query.Dimension,
			Value:     value,
			Metrics:   buildFunnelMetrics(query, facts, filter, true, rawCutoff),
		})
	}
	return response, nil
}

func QueryGeiliFunnelSummary(ctx context.Context, query FunnelSummaryQuery) (dto.GeiliFunnelSummaryResponse, error) {
	if err := validateFunnelSummaryQuery(query); err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	entries, err := model.LoadEntryVisitors(ctx, query.Environment, query.From, query.To)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	registrations, err := model.LoadIndependentRegistrations(ctx, query.From, query.To)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	topUps, _, err := model.LoadIndependentFirstTopUps(ctx, query.From, query.To)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	tasks, _, err := model.LoadIndependentFirstTasks(ctx, query.From, query.To)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	studio, err := model.LoadIndependentFirstStudio(ctx, query.Environment, query.From, query.To)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}

	userIDs := funnelSummaryUserIDs(entries, registrations, topUps, tasks, studio)
	linkedTouches, err := model.LoadLinkedFirstTouches(ctx, query.Environment, userIDs)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	created, err := model.LoadUserCreatedTimes(ctx, userIDs)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	firstTopUps, _, err := model.LoadFirstSuccessfulTopUps(ctx, userIDs)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	firstTasks, _, err := model.LoadFirstSuccessfulTasks(ctx, userIDs)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	studioOpens, err := model.LoadStudioOpenTimes(ctx, query.Environment, userIDs)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	activity, err := model.LoadFunnelActivityDays(ctx, query.Environment, userIDs, utcDay(query.From), query.To)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	failures, err := model.LoadFunnelFailures(ctx, query.Environment, query.From, query.To)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	taskStatuses, err := model.LoadTaskStatusFacts(ctx, query.From, query.To)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	collectionStartedAt, err := model.LoadFunnelCollectionStart(ctx, query.Environment)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	lastEventAt, err := model.LoadFunnelLastEventAt(ctx, query.Environment)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	invalidTopUps, invalidTasks, err := model.LoadInvalidFunnelBusinessTimes(ctx)
	if err != nil {
		return dto.GeiliFunnelSummaryResponse{}, err
	}
	counters := GetFunnelIngestCounters()
	facts := FunnelSummaryFacts{
		EntryVisitors: entries, LinkedFirstTouches: linkedTouches, UserCreated: created,
		FirstTopUps: firstTopUps, FirstTasks: firstTasks, StudioOpens: studioOpens,
		IndependentRegistrations: registrations, IndependentTopUps: topUps,
		IndependentTasks: tasks, IndependentStudio: studio, ActivityDays: activity,
		Failures: failures, TaskStatuses: taskStatuses, CollectionStartedAt: collectionStartedAt,
		LastEventAt: lastEventAt, DuplicateSinceStart: counters.Duplicate,
		RejectedSinceStart: counters.Rejected, CounterSince: counters.Since,
		InvalidTopUpTimes: invalidTopUps, InvalidTaskTimes: invalidTasks,
	}
	return BuildFunnelSummary(query, facts)
}

func buildFunnelMetrics(query FunnelSummaryQuery, facts FunnelSummaryFacts, filter *funnelSegmentFilter, suppress bool, rawCutoff int64) dto.FunnelMetrics {
	return dto.FunnelMetrics{
		Strict:      buildStrictFunnel(query, facts, filter, suppress, rawCutoff),
		Independent: buildIndependentMilestones(query, facts, filter, suppress, rawCutoff),
		Retention:   buildFunnelRetention(query, facts, filter, suppress),
		Failures:    buildFunnelFailures(query, facts, filter, suppress, rawCutoff),
	}
}

func buildStrictFunnel(query FunnelSummaryQuery, facts FunnelSummaryFacts, filter *funnelSegmentFilter, suppress bool, rawCutoff int64) dto.FunnelStrict {
	coverage := strictFunnelCoverage(query, facts.CollectionStartedAt, rawCutoff)
	names := []string{"slp_view", "registered", "first_top_up", "first_generation", "opened_studio"}
	if coverage == "unavailable" || coverage == "not_started" || coverage == "pre_collection" {
		stages := make([]dto.FunnelStage, 0, len(names))
		for _, name := range names {
			stages = append(stages, dto.FunnelStage{Name: name})
		}
		return dto.FunnelStrict{Coverage: coverage, Stages: stages}
	}

	var counts [5]int64
	for _, visitor := range facts.EntryVisitors {
		if matchesVisitorFilter(visitor, filter) {
			counts[0]++
		}
	}
	userIDs := make([]int, 0, len(facts.LinkedFirstTouches))
	for userID := range facts.LinkedFirstTouches {
		userIDs = append(userIDs, userID)
	}
	sort.Ints(userIDs)
	for _, userID := range userIDs {
		touch := facts.LinkedFirstTouches[userID]
		if touch.FirstSLPAt < query.From || touch.FirstSLPAt >= query.To || !matchesVisitorFilter(touch, filter) {
			continue
		}
		created, ok := facts.UserCreated[userID]
		if !ok || created < touch.FirstSLPAt || created >= query.To {
			continue
		}
		counts[1]++
		firstTopUp, ok := facts.FirstTopUps[userID]
		if !ok || firstTopUp < created || firstTopUp >= query.To {
			continue
		}
		counts[2]++
		firstTask, ok := facts.FirstTasks[userID]
		if !ok || firstTask < firstTopUp || firstTask >= query.To {
			continue
		}
		counts[3]++
		studioAt := firstAtOrAfter(facts.StudioOpens[userID], firstTask)
		if studioAt == 0 || studioAt >= query.To {
			continue
		}
		counts[4]++
	}

	stages := make([]dto.FunnelStage, 0, len(names))
	for index, name := range names {
		count := counts[index]
		stage := dto.FunnelStage{Name: name, FunnelCount: dto.FunnelCount{People: int64Pointer(count)}}
		if index == 0 {
			stage.RatePrevious = ratioPointer(count, count)
		} else {
			stage.RatePrevious = ratioPointer(count, counts[index-1])
		}
		stage.RateEntry = ratioPointer(count, counts[0])
		if suppress && index > 0 && count < funnelSmallCellMinimum {
			stage.People = nil
			stage.RatePrevious = nil
			stage.RateEntry = nil
			stage.Suppressed = true
		}
		stages = append(stages, stage)
	}
	return dto.FunnelStrict{Coverage: coverage, Stages: stages}
}

func buildIndependentMilestones(query FunnelSummaryQuery, facts FunnelSummaryFacts, filter *funnelSegmentFilter, suppress bool, rawCutoff int64) []dto.FunnelMilestone {
	type milestoneSource struct {
		name string
		rows []model.FunnelTimedUser
	}
	sources := []milestoneSource{
		{name: "registered", rows: facts.IndependentRegistrations},
		{name: "first_top_up", rows: facts.IndependentTopUps},
		{name: "first_generation", rows: facts.IndependentTasks},
		{name: "opened_studio", rows: facts.IndependentStudio},
	}
	result := make([]dto.FunnelMilestone, 0, len(sources))
	for index, source := range sources {
		coverage := "business_only"
		if filter != nil {
			coverage = "attributed_only"
		}
		if index == 3 && query.From < rawCutoff {
			result = append(result, dto.FunnelMilestone{Name: source.name, Ordered: false, Coverage: "unavailable"})
			continue
		}
		if index == 3 && filter == nil {
			coverage = "event_only"
		}
		count := countTimedUsers(source.rows, facts.LinkedFirstTouches, filter)
		milestone := dto.FunnelMilestone{
			Name: source.name, People: int64Pointer(count), Ordered: false, Coverage: coverage,
		}
		if suppress && count < funnelSmallCellMinimum {
			milestone.People = nil
			milestone.Suppressed = true
		}
		result = append(result, milestone)
	}
	return result
}

func buildFunnelRetention(query FunnelSummaryQuery, facts FunnelSummaryFacts, filter *funnelSegmentFilter, suppress bool) []dto.FunnelRetention {
	activity := make(map[[2]int64]struct{}, len(facts.ActivityDays))
	for _, row := range facts.ActivityDays {
		activity[[2]int64{int64(row.UserID), row.ActivityDate}] = struct{}{}
	}
	observationEnd := query.To
	if today := utcDay(query.Now); today < observationEnd {
		observationEnd = today
	}
	result := make([]dto.FunnelRetention, 0, 3)
	for _, day := range []int{1, 7, 30} {
		var eligible, retained, immature int64
		seen := make(map[int]struct{})
		for _, registration := range facts.IndependentRegistrations {
			if _, ok := seen[registration.UserID]; ok {
				continue
			}
			touch, linked := facts.LinkedFirstTouches[registration.UserID]
			if !linked || touch.IdentityState != model.FunnelIdentityLinked || !matchesVisitorFilter(touch, filter) {
				continue
			}
			seen[registration.UserID] = struct{}{}
			target := utcDay(registration.At) + int64(day)*86400
			if target >= observationEnd {
				immature++
				continue
			}
			eligible++
			if _, ok := activity[[2]int64{int64(registration.UserID), target}]; ok {
				retained++
			}
		}
		row := dto.FunnelRetention{
			Day: day, Eligible: int64Pointer(eligible), Retained: int64Pointer(retained),
			Rate: ratioPointer(retained, eligible), Immature: int64Pointer(immature),
			Coverage: "web_linked_only",
		}
		if suppress {
			if eligible < funnelSmallCellMinimum {
				row.Eligible = nil
				row.Rate = nil
				row.Suppressed = true
			}
			if retained < funnelSmallCellMinimum {
				row.Retained = nil
				row.Rate = nil
				row.Suppressed = true
			}
			if immature < funnelSmallCellMinimum {
				row.Immature = nil
				row.Suppressed = true
			}
		}
		result = append(result, row)
	}
	return result
}

func buildFunnelFailures(query FunnelSummaryQuery, facts FunnelSummaryFacts, filter *funnelSegmentFilter, suppress bool, rawCutoff int64) []dto.FunnelFailure {
	result := make([]dto.FunnelFailure, 0)
	if query.From >= rawCutoff {
		counts := make(map[string]int64)
		for _, row := range facts.Failures {
			if _, allowed := funnelFailureCodes[row.FailureCode]; !allowed {
				continue
			}
			if filter != nil {
				if filter.dimension != "model" || row.ModelSlug != filter.value {
					continue
				}
			}
			counts[row.FailureCode] += row.Count
		}
		codes := make([]string, 0, len(counts))
		for code := range counts {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			count := counts[code]
			row := dto.FunnelFailure{Code: code, Count: int64Pointer(count), Coverage: "event_only"}
			if filter != nil {
				row.Coverage = "attributed_only"
				if filter.dimension == "model" {
					row.Model = filter.value
				}
			}
			if suppress && count < funnelSmallCellMinimum {
				row.Count = nil
				row.Suppressed = true
			}
			result = append(result, row)
		}
	}
	if filter != nil {
		return result
	}
	taskCounts := map[model.TaskStatus]int64{}
	for _, row := range facts.TaskStatuses {
		taskCounts[row.Status] += row.Count
	}
	result = append(result,
		dto.FunnelFailure{Code: "task_failure", Count: int64Pointer(taskCounts[model.TaskStatusFailure]), Coverage: "business_only"},
		dto.FunnelFailure{Code: "task_success", Count: int64Pointer(taskCounts[model.TaskStatusSuccess]), Coverage: "business_only"},
	)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Code == result[j].Code {
			return result[i].Model < result[j].Model
		}
		return result[i].Code < result[j].Code
	})
	return result
}

func strictFunnelCoverage(query FunnelSummaryQuery, collectionStartedAt, rawCutoff int64) string {
	if query.From < rawCutoff {
		return "unavailable"
	}
	if collectionStartedAt == 0 {
		return "not_started"
	}
	if query.To <= collectionStartedAt {
		return "pre_collection"
	}
	if query.From < collectionStartedAt && collectionStartedAt < query.To {
		return "collection_limited"
	}
	return "full"
}

func validateFunnelSummaryQuery(query FunnelSummaryQuery) error {
	if query.Environment != model.FunnelEnvironmentProduction && query.Environment != model.FunnelEnvironmentStaging {
		return ErrInvalidFunnelSummaryQuery
	}
	if query.Dimension != "all" && query.Dimension != "locale" && query.Dimension != "model" {
		return ErrInvalidFunnelSummaryQuery
	}
	if query.From < 0 || query.To <= query.From || query.Now <= 0 {
		return ErrInvalidFunnelSummaryQuery
	}
	return nil
}

func funnelSummaryUserIDs(entries []model.FunnelVisitorFact, lists ...[]model.FunnelTimedUser) []int {
	set := make(map[int]struct{})
	for _, entry := range entries {
		if entry.UserID != nil && *entry.UserID > 0 {
			set[*entry.UserID] = struct{}{}
		}
	}
	for _, list := range lists {
		for _, row := range list {
			if row.UserID > 0 {
				set[row.UserID] = struct{}{}
			}
		}
	}
	result := make([]int, 0, len(set))
	for userID := range set {
		result = append(result, userID)
	}
	sort.Ints(result)
	return result
}

func countTimedUsers(rows []model.FunnelTimedUser, touches map[int]model.FunnelVisitorFact, filter *funnelSegmentFilter) int64 {
	seen := make(map[int]struct{})
	for _, row := range rows {
		if row.UserID <= 0 {
			continue
		}
		if filter != nil {
			touch, ok := touches[row.UserID]
			if !ok || touch.IdentityState != model.FunnelIdentityLinked || !matchesVisitorFilter(touch, filter) {
				continue
			}
		}
		seen[row.UserID] = struct{}{}
	}
	return int64(len(seen))
}

func matchesVisitorFilter(visitor model.FunnelVisitorFact, filter *funnelSegmentFilter) bool {
	if filter == nil {
		return true
	}
	return visitorDimension(visitor, filter.dimension) == filter.value
}

func visitorDimension(visitor model.FunnelVisitorFact, dimension string) string {
	switch dimension {
	case "locale":
		return visitor.Locale
	case "model":
		return visitor.ModelSlug
	default:
		return "all"
	}
}

func firstAtOrAfter(values []int64, threshold int64) int64 {
	for _, value := range values {
		if value >= threshold {
			return value
		}
	}
	return 0
}

func utcDay(value int64) int64 {
	return value - value%86400
}

func ratioPointer(numerator, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := math.Round((float64(numerator)/float64(denominator))*10000) / 10000
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}
