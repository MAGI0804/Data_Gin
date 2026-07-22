package job

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/hibiken/asynq"
)

const (
	TypeMallGeocode          = "mall:geocode"
	TypeMallWeatherFast      = "mall:weather:fast"
	TypeMallWeatherFull      = "mall:weather:full"
	TypeMallWeatherLifeIndex = "mall:weather:life_index"
	TypeMallWeatherRepair    = "mall:weather:repair"
	TypeMallWeatherManual    = "mall:weather:manual"
	TypeMallWeatherExport    = "mall:weather:export"
	TypeMallWeatherFeishu    = "mall:weather:feishu"

	MallWeatherQueueName  = "weather"
	MallExportQueueName   = "export"
	MallDeliveryQueueName = "delivery"
)

var mallWeatherTaskTypes = []string{
	TypeMallGeocode,
	TypeMallWeatherFast,
	TypeMallWeatherFull,
	TypeMallWeatherLifeIndex,
	TypeMallWeatherRepair,
	TypeMallWeatherManual,
	TypeMallWeatherExport,
	TypeMallWeatherFeishu,
}

// MallTaskPayload deliberately carries only the database identifier and
// non-sensitive idempotency window needed by weather workers. Provider
// credentials are resolved by the worker.
type MallTaskPayload struct {
	MallID     uint   `json:"mall_id"`
	TaskWindow string `json:"task_window"`
}

type MallGeocodeTaskPayload struct {
	MallID      uint   `json:"mall_id"`
	MallVersion uint64 `json:"mall_version"`
	AddressHash string `json:"address_hash"`
}

var sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var mallWeatherTaskWindowPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9:_-]{0,63}$`)

// MallWeatherExportTaskPayload identifies a persisted export job.
type MallWeatherExportTaskPayload struct {
	ExportJobID uint `json:"export_job_id"`
}

// MallWeatherFeishuTaskPayload identifies a persisted pipeline run. Feishu
// tokens and document resource identifiers are never copied into Asynq.
type MallWeatherFeishuTaskPayload struct {
	PipelineRunID uint `json:"pipeline_run_id"`
}

func MallWeatherTaskTypes() []string {
	return append([]string(nil), mallWeatherTaskTypes...)
}

func ExpectedMallWeatherQueue(taskType string) (string, bool) {
	switch taskType {
	case TypeMallGeocode,
		TypeMallWeatherFast,
		TypeMallWeatherFull,
		TypeMallWeatherLifeIndex,
		TypeMallWeatherRepair,
		TypeMallWeatherManual:
		return MallWeatherQueueName, true
	case TypeMallWeatherExport:
		return MallExportQueueName, true
	case TypeMallWeatherFeishu:
		return MallDeliveryQueueName, true
	default:
		return "", false
	}
}

func NewMallGeocodeTask(payload MallGeocodeTaskPayload) (*asynq.Task, error) {
	return newTypedMallWeatherTask(TypeMallGeocode, payload)
}

func NewMallWeatherFastTask(payload MallTaskPayload) (*asynq.Task, error) {
	return newTypedMallWeatherTask(TypeMallWeatherFast, payload)
}

func NewMallWeatherFullTask(payload MallTaskPayload) (*asynq.Task, error) {
	return newTypedMallWeatherTask(TypeMallWeatherFull, payload)
}

func NewMallWeatherLifeIndexTask(payload MallTaskPayload) (*asynq.Task, error) {
	return newTypedMallWeatherTask(TypeMallWeatherLifeIndex, payload)
}

func NewMallWeatherRepairTask(payload MallTaskPayload) (*asynq.Task, error) {
	return newTypedMallWeatherTask(TypeMallWeatherRepair, payload)
}

func NewMallWeatherManualTask(payload MallTaskPayload) (*asynq.Task, error) {
	return newTypedMallWeatherTask(TypeMallWeatherManual, payload)
}

func NewMallWeatherExportTask(payload MallWeatherExportTaskPayload) (*asynq.Task, error) {
	return newTypedMallWeatherTask(TypeMallWeatherExport, payload)
}

func NewMallWeatherFeishuTask(payload MallWeatherFeishuTaskPayload) (*asynq.Task, error) {
	return newTypedMallWeatherTask(TypeMallWeatherFeishu, payload)
}

func newTypedMallWeatherTask(taskType string, payload interface{}) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("mall weather task: marshal payload: %w", err)
	}
	return NewMallWeatherTask(taskType, data)
}

// NewMallWeatherTask validates the persisted payload before it can cross the
// database-to-queue boundary. Unknown fields are rejected so credentials and
// third-party URLs cannot be smuggled into a task payload.
func NewMallWeatherTask(taskType string, payload []byte) (*asynq.Task, error) {
	queue, ok := ExpectedMallWeatherQueue(taskType)
	if !ok {
		return nil, fmt.Errorf("mall weather task: unsupported task type")
	}

	switch taskType {
	case TypeMallGeocode:
		if _, err := DecodeMallGeocodeTaskPayload(payload); err != nil {
			return nil, err
		}
	case TypeMallWeatherFast,
		TypeMallWeatherFull,
		TypeMallWeatherLifeIndex,
		TypeMallWeatherRepair,
		TypeMallWeatherManual:
		if _, err := DecodeMallWeatherTaskPayload(taskType, payload); err != nil {
			return nil, err
		}
	case TypeMallWeatherExport:
		var decoded MallWeatherExportTaskPayload
		if err := decodeStrictTaskPayload(payload, &decoded); err != nil {
			return nil, err
		}
		if decoded.ExportJobID == 0 {
			return nil, fmt.Errorf("mall weather task: export_job_id is required")
		}
	case TypeMallWeatherFeishu:
		var decoded MallWeatherFeishuTaskPayload
		if err := decodeStrictTaskPayload(payload, &decoded); err != nil {
			return nil, err
		}
		if decoded.PipelineRunID == 0 {
			return nil, fmt.Errorf("mall weather task: pipeline_run_id is required")
		}
	}

	return asynq.NewTask(taskType, append([]byte(nil), payload...), asynq.Queue(queue)), nil
}

func DecodeMallWeatherTaskPayload(taskType string, payload []byte) (MallTaskPayload, error) {
	prefix, ok := mallWeatherTaskWindowPrefix(taskType)
	if !ok {
		return MallTaskPayload{}, fmt.Errorf("mall weather task: unsupported weather task type")
	}
	var decoded MallTaskPayload
	if err := decodeStrictTaskPayload(payload, &decoded); err != nil {
		return MallTaskPayload{}, err
	}
	if decoded.MallID == 0 || !mallWeatherTaskWindowPattern.MatchString(decoded.TaskWindow) ||
		!strings.HasPrefix(decoded.TaskWindow, prefix) || len(decoded.TaskWindow) <= len(prefix) ||
		!weatherWindowMatchesMall(taskType, decoded) {
		return MallTaskPayload{}, fmt.Errorf("mall weather task: invalid weather task identity")
	}
	return decoded, nil
}

func weatherWindowMatchesMall(taskType string, payload MallTaskPayload) bool {
	switch taskType {
	case TypeMallWeatherFast, TypeMallWeatherFull, TypeMallWeatherLifeIndex:
		prefix, _ := mallWeatherTaskWindowPrefix(taskType)
		return strings.HasPrefix(payload.TaskWindow, prefix+strconv.FormatUint(uint64(payload.MallID), 10)+":")
	default:
		return true
	}
}

func mallWeatherTaskWindowPrefix(taskType string) (string, bool) {
	switch taskType {
	case TypeMallWeatherFast:
		return "fast:", true
	case TypeMallWeatherFull:
		return "full:", true
	case TypeMallWeatherLifeIndex:
		return "life:", true
	case TypeMallWeatherRepair:
		return "repair:", true
	case TypeMallWeatherManual:
		return "manual:", true
	default:
		return "", false
	}
}

func DecodeMallGeocodeTaskPayload(payload []byte) (MallGeocodeTaskPayload, error) {
	var decoded MallGeocodeTaskPayload
	if err := decodeStrictTaskPayload(payload, &decoded); err != nil {
		return MallGeocodeTaskPayload{}, err
	}
	if decoded.MallID == 0 || decoded.MallVersion == 0 || !sha256HexPattern.MatchString(decoded.AddressHash) {
		return MallGeocodeTaskPayload{}, fmt.Errorf("mall weather task: invalid geocode identity")
	}
	return decoded, nil
}

func decodeStrictTaskPayload(payload []byte, destination interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("mall weather task: invalid payload schema: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("mall weather task: payload must contain one json object")
	}
	return nil
}
