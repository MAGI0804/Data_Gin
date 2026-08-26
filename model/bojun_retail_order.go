package model

import "time"

// BojunRetailOrder 伯俊零售单头数据。
type BojunRetailOrder struct {
	BaseModel

	RawDataID      uint    `gorm:"column:raw_data_id;not null;index" json:"raw_data_id"`
	OracleRetailID *uint64 `gorm:"column:oracle_retail_id;uniqueIndex" json:"oracle_retail_id,omitempty"`
	OrderPhone     string  `gorm:"column:order_phone;size:64;index" json:"order_phone"`
	PaidAmount     float64 `gorm:"column:paid_amount;type:decimal(18,2);default:0" json:"paid_amount"`
	PushAmount     float64 `gorm:"column:push_amount;type:decimal(18,2);default:0" json:"push_amount"`
	IsToShop       string  `gorm:"column:is_to_shop;size:1;index" json:"is_to_shop"`

	OtherDocNo      string     `gorm:"column:otherdocno;size:255" json:"otherdocno"`
	DocNo           string     `gorm:"column:docno;size:255;not null;uniqueIndex" json:"docno"`
	BillDate        int        `gorm:"column:billdate;index" json:"billdate"`
	CompletedAt     *time.Time `gorm:"column:completed_at;type:datetime" json:"completedAt,omitempty"`
	RetailBillType  string     `gorm:"column:retailbilltype;size:50;index" json:"retailbilltype"`
	StoreCode       string     `gorm:"column:c_store_code;size:100;index" json:"cStoreCode"`
	StoreName       string     `gorm:"column:c_store_name;size:255" json:"cStoreName"`
	UploadType      string     `gorm:"column:uploadtype;size:100" json:"uploadtype"`
	VIPNo           string     `gorm:"column:vipno;size:100;index" json:"vipno"`
	RetailTypeName  string     `gorm:"column:c_retailtype_name;size:100" json:"cRetailtypeName"`
	SalesRep        string     `gorm:"column:salesrep;size:100" json:"salesrep"`
	IsDiscount      string     `gorm:"column:is_dis;size:20" json:"isDis"`
	VouchersNo      string     `gorm:"column:vouchers_no;size:255" json:"vouchersNo"`
	IsIntegral      string     `gorm:"column:isintl;size:20" json:"isintl"`
	DocNoIntegral   int        `gorm:"column:docno_integral;default:0" json:"docnoIntegral"`
	OrderMark       string     `gorm:"column:ordermark;size:255" json:"ordermark"`
	RetailSaleType  string     `gorm:"column:retailsaletype;size:50;index" json:"retailsaletype"`
	OrderTypeCode   string     `gorm:"column:order_type_code;size:50;index" json:"order_type_code"`
	OrderTypeName   string     `gorm:"column:order_type_name;size:50;index" json:"order_type_name"`
	Description     string     `gorm:"column:description;type:text" json:"description"`
	TotalLines      int        `gorm:"column:tot_lines;default:0" json:"totLines"`
	O2OSoDocNo      string     `gorm:"column:o2o_so_docno;size:255" json:"o2oSoDocno"`
	TotalQty        int        `gorm:"column:tot_qty;default:0" json:"totQty"`
	TotalAmtList    float64    `gorm:"column:tot_amt_list;type:decimal(18,2);default:0" json:"totAmtList"`
	TotalAmtActual  float64    `gorm:"column:tot_amt_actual;type:decimal(18,2);default:0" json:"totAmtActual"`
	AvgDiscount     float64    `gorm:"column:avg_discount;type:decimal(10,4);default:0" json:"avgDiscount"`
	TotalAmtAcc     float64    `gorm:"column:tot_amt_acc;type:decimal(18,2);default:0" json:"totAmtAcc"`
	TotalAmtAcc1    float64    `gorm:"column:tot_amt_acc1;type:decimal(18,2);default:0" json:"totAmtAcc1"`
	OzID            string     `gorm:"column:ozid;size:255" json:"ozid"`
	RelatedNormalNo string     `gorm:"column:related_normal_docno;size:255;index" json:"related_normal_docno"`
	MatchedDocNo    string     `gorm:"column:matched_docno;size:255;index" json:"matched_docno"`

	ItemsJSON      string `gorm:"column:items_json;type:json" json:"items_json"`
	PayItemsJSON   string `gorm:"column:pay_items_json;type:json" json:"pay_items_json"`
	RawContentJSON string `gorm:"column:raw_content_json;type:json" json:"raw_content_json"`

	Synced int `gorm:"column:synced;not null;default:0;index" json:"synced"`

	CommonTimestampsField
}

func (BojunRetailOrder) TableName() string {
	return "bojun_retail_orders"
}

// BojunOracleSyncState records the committed M_RETAIL_ID watermark for one
// Oracle source. A state row is created only when the Oracle sync is explicitly
// initialized; adding this table does not activate the Oracle order source.
type BojunOracleSyncState struct {
	BaseModel
	SourceCode   string `gorm:"column:source_code;size:64;not null;uniqueIndex" json:"source_code"`
	LastRetailID uint64 `gorm:"column:last_retail_id;not null;default:0" json:"last_retail_id"`
	Initialized  bool   `gorm:"column:initialized;not null;default:false" json:"initialized"`
	CommonTimestampsField
}

func (BojunOracleSyncState) TableName() string {
	return "bojun_oracle_sync_states"
}
