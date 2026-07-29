package data_svc

import "fmt"

const maxOpenCursorPage = 1_000_000

type OpenPagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	TotalItems int64 `json:"totalItems"`
	TotalPages int64 `json:"totalPages"`
}

func newOpenPagination(page, pageSize int, totalItems int64) OpenPagination {
	if page < 1 {
		page = 1
	}
	totalPages := int64(0)
	if pageSize > 0 && totalItems > 0 {
		totalPages = (totalItems + int64(pageSize) - 1) / int64(pageSize)
	}
	return OpenPagination{Page: page, PageSize: pageSize, TotalItems: totalItems, TotalPages: totalPages}
}

func openCursorPage(cursorPage int, hasCursor bool) int {
	if !hasCursor {
		return 1
	}
	if cursorPage < 2 {
		return 2
	}
	return cursorPage
}

func invalidOpenCursorPage(page int) bool {
	return page < 0 || page == 1 || page > maxOpenCursorPage
}

func nextOpenCursorPage(page int) (int, error) {
	if page < 1 || page >= maxOpenCursorPage {
		return 0, fmt.Errorf("open pagination: page limit exceeded")
	}
	return page + 1, nil
}

func applyOpenWeatherPagination(pagination *MallWeatherPagination, page int, totalItems int64) {
	if pagination == nil {
		return
	}
	summary := newOpenPagination(page, pagination.PageSize, totalItems)
	pagination.Page = summary.Page
	pagination.TotalItems = &summary.TotalItems
	pagination.TotalPages = &summary.TotalPages
}
