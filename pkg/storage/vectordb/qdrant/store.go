package qdrant

import (
	"context"
	"fmt"

	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"abot/pkg/types"
)

// Store implements types.VectorStore backed by Qdrant.
type Store struct {
	conn       *grpc.ClientConn
	points     pb.PointsClient
	collections pb.CollectionsClient
	dimension  uint64
}

// Config holds Qdrant connection parameters.
type Config struct {
	Addr      string // e.g. "localhost:6334"
	Dimension int
}

// New connects to Qdrant and returns a Store.
func New(cfg Config) (*Store, error) {
	conn, err := grpc.NewClient(cfg.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("qdrant dial: %w", err)
	}
	return &Store{
		conn:        conn,
		points:      pb.NewPointsClient(conn),
		collections: pb.NewCollectionsClient(conn),
		dimension:   uint64(cfg.Dimension),
	}, nil
}

func (s *Store) Close() error {
	return s.conn.Close()
}

func (s *Store) EnsureCollection(ctx context.Context, collection string) error {
	_, err := s.collections.Get(ctx, &pb.GetCollectionInfoRequest{
		CollectionName: collection,
	})
	if err == nil {
		return nil // already exists
	}
	_, err = s.collections.Create(ctx, &pb.CreateCollection{
		CollectionName: collection,
		VectorsConfig: &pb.VectorsConfig{
			Config: &pb.VectorsConfig_Params{
				Params: &pb.VectorParams{
					Size:     s.dimension,
					Distance: pb.Distance_Cosine,
				},
			},
		},
	})
	return err
}

func (s *Store) Upsert(ctx context.Context, collection string, entries []types.VectorEntry) error {
	points := make([]*pb.PointStruct, len(entries))
	for i, e := range entries {
		points[i] = &pb.PointStruct{
			Id:      &pb.PointId{PointIdOptions: &pb.PointId_Uuid{Uuid: e.ID}},
			Vectors: &pb.Vectors{VectorsOptions: &pb.Vectors_Vector{Vector: &pb.Vector{Data: e.Vector}}},
			Payload: toQdrantPayload(e.Payload),
		}
	}
	_, err := s.points.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: collection,
		Points:         points,
	})
	return err
}

func (s *Store) Search(ctx context.Context, collection string, req *types.VectorSearchRequest) ([]types.VectorResult, error) {
	limit := uint64(req.TopK)
	resp, err := s.points.Search(ctx, &pb.SearchPoints{
		CollectionName: collection,
		Vector:         req.Vector,
		Limit:          limit,
		Filter:         buildFilter(req.Filter),
		WithPayload:    &pb.WithPayloadSelector{SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true}},
	})
	if err != nil {
		return nil, err
	}
	out := make([]types.VectorResult, len(resp.Result))
	for i, r := range resp.Result {
		out[i] = types.VectorResult{
			ID:      r.Id.GetUuid(),
			Score:   r.Score,
			Payload: fromQdrantPayload(r.Payload),
		}
	}
	return out, nil
}

func (s *Store) Delete(ctx context.Context, collection string, filter map[string]any) error {
	_, err := s.points.Delete(ctx, &pb.DeletePoints{
		CollectionName: collection,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Filter{
				Filter: buildFilter(filter),
			},
		},
	})
	return err
}

func (s *Store) UpdatePayload(ctx context.Context, collection string, filter map[string]any, payload map[string]any) error {
	f := buildFilter(filter)
	if f == nil {
		return fmt.Errorf("UpdatePayload requires a non-empty filter")
	}
	_, err := s.points.SetPayload(ctx, &pb.SetPayloadPoints{
		CollectionName: collection,
		Payload:        toQdrantPayload(payload),
		PointsSelector: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Filter{Filter: f},
		},
	})
	return err
}

// --- helpers ---

func toQdrantPayload(m map[string]any) map[string]*pb.Value {
	out := make(map[string]*pb.Value, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case string:
			out[k] = &pb.Value{Kind: &pb.Value_StringValue{StringValue: val}}
		case float64:
			out[k] = &pb.Value{Kind: &pb.Value_DoubleValue{DoubleValue: val}}
		case int:
			out[k] = &pb.Value{Kind: &pb.Value_IntegerValue{IntegerValue: int64(val)}}
		case bool:
			out[k] = &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: val}}
		}
	}
	return out
}

func fromQdrantPayload(m map[string]*pb.Value) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.Kind.(type) {
		case *pb.Value_StringValue:
			out[k] = val.StringValue
		case *pb.Value_DoubleValue:
			out[k] = val.DoubleValue
		case *pb.Value_IntegerValue:
			out[k] = val.IntegerValue
		case *pb.Value_BoolValue:
			out[k] = val.BoolValue
		}
	}
	return out
}

func buildFilter(m map[string]any) *pb.Filter {
	if len(m) == 0 {
		return nil
	}
	var must []*pb.Condition
	for k, v := range m {
		switch val := v.(type) {
		case string:
			must = append(must, &pb.Condition{
				ConditionOneOf: &pb.Condition_Field{
					Field: &pb.FieldCondition{
						Key: k,
						Match: &pb.Match{
							MatchValue: &pb.Match_Keyword{Keyword: val},
						},
					},
				},
			})
		case bool:
			must = append(must, &pb.Condition{
				ConditionOneOf: &pb.Condition_Field{
					Field: &pb.FieldCondition{
						Key: k,
						Match: &pb.Match{
							MatchValue: &pb.Match_Boolean{Boolean: val},
						},
					},
				},
			})
		}
	}
	return &pb.Filter{Must: must}
}

var _ types.VectorStore = (*Store)(nil)