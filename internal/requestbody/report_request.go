package requestbody

import "encoding/json"

// ReportDraftSaveRequest is the complete mutable report contract. Updates
// replace the draft atomically and require ExpectedLockVersion.
type ReportDraftSaveRequest struct {
	Code                string                   `json:"code"`
	Name                string                   `json:"name"`
	Category            string                   `json:"category,omitempty"`
	Description         string                   `json:"description,omitempty"`
	DatasourceID        uint                     `json:"datasourceId"`
	ExpectedLockVersion *uint64                  `json:"expectedLockVersion,omitempty"`
	Procedure           ReportProcedureRequest   `json:"procedure"`
	Result              ReportResultRequest      `json:"result"`
	CallTemplate        string                   `json:"callTemplate"`
	Parameters          []ReportParameterRequest `json:"parameters"`
	Columns             []ReportColumnRequest    `json:"columns"`
	Grants              []ReportGrantRequest     `json:"grants"`
}

type ReportProcedureRequest struct {
	Owner    string `json:"owner"`
	Package  string `json:"package,omitempty"`
	Name     string `json:"name"`
	Overload string `json:"overload,omitempty"`
}

type ReportResultRequest struct {
	TableOwner  string `json:"tableOwner"`
	TableName   string `json:"tableName"`
	RunIDColumn string `json:"runIdColumn"`
	RowIDColumn string `json:"rowIdColumn"`
}

type ReportParameterRequest struct {
	Code               string          `json:"code"`
	Label              string          `json:"label"`
	DisplayOrder       int             `json:"displayOrder"`
	ControlType        string          `json:"controlType"`
	LogicalType        string          `json:"logicalType"`
	Cardinality        string          `json:"cardinality"`
	ProcedureArgName   string          `json:"procedureArgName"`
	Position           int             `json:"position"`
	OracleType         string          `json:"oracleType"`
	Precision          *int            `json:"precision,omitempty"`
	Scale              *int            `json:"scale,omitempty"`
	MaxLength          *int            `json:"maxLength,omitempty"`
	Required           bool            `json:"required"`
	Nullable           bool            `json:"nullable"`
	SystemInjected     bool            `json:"systemInjected"`
	Sensitive          bool            `json:"sensitive"`
	DefaultValue       json.RawMessage `json:"defaultValue,omitempty"`
	AllowedValues      json.RawMessage `json:"allowedValues,omitempty"`
	Validation         json.RawMessage `json:"validation,omitempty"`
	Normalizer         json.RawMessage `json:"normalizer,omitempty"`
	ValueSource        json.RawMessage `json:"valueSource,omitempty"`
	Timezone           string          `json:"timezone,omitempty"`
	NullPolicy         string          `json:"nullPolicy"`
	CollectionEncoding string          `json:"collectionEncoding,omitempty"`
	ErrorMessage       string          `json:"errorMessage,omitempty"`
}

type ReportColumnRequest struct {
	FieldID           string          `json:"fieldId"`
	LogicalCode       string          `json:"logicalCode"`
	DatabaseColumn    string          `json:"databaseColumn"`
	SourceOracleType  string          `json:"sourceOracleType"`
	Precision         *int            `json:"precision,omitempty"`
	Scale             *int            `json:"scale,omitempty"`
	Nullable          bool            `json:"nullable"`
	ValueType         string          `json:"valueType"`
	PreviewHeader     string          `json:"previewHeader"`
	ExcelHeader       string          `json:"excelHeader"`
	DisplayOrder      int             `json:"displayOrder"`
	ExportOrder       int             `json:"exportOrder"`
	PreviewVisible    bool            `json:"previewVisible"`
	ExportVisible     bool            `json:"exportVisible"`
	Filterable        bool            `json:"filterable"`
	Sortable          bool            `json:"sortable"`
	ExportAllowed     bool            `json:"exportAllowed"`
	AllowedOperators  json.RawMessage `json:"allowedOperators,omitempty"`
	Format            json.RawMessage `json:"format,omitempty"`
	DictionaryVersion json.RawMessage `json:"dictionaryVersion,omitempty"`
	MaskingPolicy     json.RawMessage `json:"maskingPolicy,omitempty"`
	ExcelWidth        float64         `json:"excelWidth"`
	NullDisplay       string          `json:"nullDisplay,omitempty"`
}

type ReportGrantRequest struct {
	SubjectType string          `json:"subjectType"`
	SubjectID   uint            `json:"subjectId"`
	Actions     json.RawMessage `json:"actions"`
}
