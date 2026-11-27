package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

var (
	quoteStatuses = []string{"draft", "sent", "accepted", "rejected", "cancelled", "expired"}
	quoteItem     = obj([]string{"name"}, map[string]any{
		"name":            sProp("Item name"),
		"description":     sProp("Details"),
		"unit":            sProp("Unit, e.g. gói, giờ, tháng"),
		"quantity":        map[string]any{"type": "number", "description": "Quantity, default 1"},
		"unitPrice":       iProp("Unit price in minor units of the currency (VND: đồng; USD: cents)"),
		"discountPercent": map[string]any{"type": "number", "description": "Line discount 0..100"},
	})
	quoteFields = map[string]any{
		"title":           sProp("Subject (V/v)"),
		"customerId":      sProp("Customer ID from list_customers; empty to clear"),
		"currency":        sProp("ISO code, e.g. VND, USD"),
		"language":        enumProp("Document language", "vi", "en"),
		"issueDate":       sProp("YYYY-MM-DD"),
		"validUntil":      sProp("YYYY-MM-DD, empty for none"),
		"discountType":    enumProp("Whole-quote discount kind (amount with 0 for none)", "percent", "amount"),
		"discountPercent": map[string]any{"type": "number", "description": "When discountType is percent, 0..100"},
		"discountAmount":  iProp("When discountType is amount, in minor units"),
		"vatRate":         map[string]any{"type": "number", "description": "VAT %, 0..100"},
		"notes":           sProp("Notes"),
		"terms":           sProp("Terms & conditions"),
		"items":           arrProp("Line items. REPLACES the whole list: send every line to keep.", quoteItem),
	}
	customerFields = map[string]any{
		"name": sProp("Company or person name"), "taxCode": sProp("Tax code (MST)"), "address": sProp("Address"),
		"contactName": sProp("Contact person"), "email": sProp("Email"), "phone": sProp("Phone"), "note": sProp("Internal note"),
	}
	quoteSettingFields = map[string]any{
		"companyName": sProp("Issuer company name"), "address": sProp("Address"), "taxCode": sProp("Tax code"),
		"phone": sProp("Phone"), "email": sProp("Email"), "website": sProp("Website"), "bankInfo": sProp("Bank transfer details"),
		"numberPrefix": sProp("Quote number prefix, e.g. BG"), "defaultCurrency": sProp("ISO code"),
		"defaultVatRate":   map[string]any{"type": "number", "description": "Default VAT %"},
		"defaultValidDays": iProp("Default validity in days"), "defaultTerms": sProp("Default terms"),
	}
)

func wsQuote(path string) string { return "/v1/workspaces/{workspaceId}/quotes" + path }

func quoteTools() []tool {
	ids := func(extra map[string]any) map[string]any {
		m := map[string]any{"workspaceId": sProp("Workspace ID"), "quoteId": sProp("Quote ID")}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	return []tool{
		{
			name:        "list_quotes",
			description: "Quotes (báo giá) of a workspace, newest first, latest version of each only (older versions are " +
				"listed in get_quote). Each quote belongs to a project module.",
			schema: obj([]string{"workspaceId"}, withPage(map[string]any{
				"workspaceId": sProp("Workspace ID"), "projectId": sProp("Only this project"),
				"moduleId": sProp("Only this module"), "status": enumProp("Only this status", quoteStatuses...),
			})),
			run: endpoint(http.MethodGet, wsQuote(""), []string{"projectId", "moduleId", "status"}, 50, nil),
		},
		{
			name: "get_quote",
			description: "Full quote: items, totals, customer, issuer, versions, signatures and history. Money " +
				"values are integers in minor units (see minorUnits); editable is true only for drafts.",
			schema: obj([]string{"workspaceId", "quoteId"}, ids(nil)),
			run:    endpoint(http.MethodGet, wsQuote("/{quoteId}"), nil, 0, nil),
		},
		{
			name: "create_quote",
			description: "Create a draft quote for a project module (defaults come from the quote settings). Any " +
				"other field given (title, items, ...) is filled in right away.",
			schema: obj([]string{"workspaceId", "moduleId"}, merge(map[string]any{
				"workspaceId": sProp("Workspace ID"), "moduleId": sProp("Module the quote is for (list_modules)"),
			}, quoteFields)),
			run: createQuote,
		},
		{
			name: "update_quote",
			description: "Change a draft quote. Only the given fields change; items replaces the whole list " +
				"(call get_quote first to keep existing lines). Sent quotes are locked: use new_quote_version.",
			schema: obj([]string{"workspaceId", "quoteId"}, ids(quoteFields)),
			run:    updateQuote,
		},
		{
			name:        "duplicate_quote",
			description: "Copy a quote as a new draft with a new number.",
			schema:      obj([]string{"workspaceId", "quoteId"}, ids(nil)),
			run:         endpoint(http.MethodPost, wsQuote("/{quoteId}/duplicate"), nil, 0, nil),
		},
		{
			name:        "new_quote_version",
			description: "Start the next version (v2, v3...) of a sent quote as an editable draft with the same number.",
			schema:      obj([]string{"workspaceId", "quoteId"}, ids(nil)),
			run:         endpoint(http.MethodPost, wsQuote("/{quoteId}/new-version"), nil, 0, nil),
		},
		{
			name: "send_quote",
			description: "Mark a draft quote as sent to the customer. This locks it (no more edits) and snapshots " +
				"the customer and issuer. It sends no email. Needs a customer and at least one item.",
			schema: obj([]string{"workspaceId", "quoteId"}, ids(nil)),
			run:    endpoint(http.MethodPost, wsQuote("/{quoteId}/send"), nil, 0, nil),
		},
		{
			name:        "set_quote_status",
			description: "Record the customer's decision on a sent quote (accepted / rejected), or cancel a draft or sent quote.",
			schema: obj([]string{"workspaceId", "quoteId", "status"}, ids(map[string]any{
				"status": enumProp("New status", "accepted", "rejected", "cancelled"),
			})),
			run: endpoint(http.MethodPost, wsQuote("/{quoteId}/status"), nil, 0, func(a args) (any, error) {
				return map[string]any{"status": a.str("status")}, nil
			}),
		},
		{
			name:        "delete_quote",
			description: "Delete a draft quote. Sent quotes cannot be deleted (cancel them instead).",
			schema:      obj([]string{"workspaceId", "quoteId"}, ids(nil)),
			run:         endpoint(http.MethodDelete, wsQuote("/{quoteId}"), nil, 0, nil),
			destructive: true,
		},
		{
			name: "export_quote",
			description: "Download a quote as a PDF or Excel file. template admin lays it out as a Vietnamese " +
				"administrative document (Nghị định 30); modern is the branded layout.",
			schema: obj([]string{"workspaceId", "quoteId"}, ids(map[string]any{
				"format":   enumProp("File type, default pdf", "pdf", "xlsx"),
				"template": enumProp("Layout, default modern", "modern", "admin"),
				"sign":     enumProp("Admin layout signature boxes: digital (stamped when signed) or wet (blank for ink)", "digital", "wet"),
				"place":    sProp("Admin layout: place in the date line, e.g. Hà Nội"),
				"signer":   sProp("Admin layout: approver title, default GIÁM ĐỐC"),
				"merge":    bProp("Admin layout: one signature block when the director prepares the quote"),
			})),
			run: exportQuote,
		},
		{
			name:        "list_customers",
			description: "Customers of a workspace (for quotes).",
			schema:      obj([]string{"workspaceId"}, map[string]any{"workspaceId": sProp("Workspace ID")}),
			run:         endpoint(http.MethodGet, "/v1/workspaces/{workspaceId}/customers", nil, 0, nil),
		},
		{
			name: "save_customer",
			description: "Create a customer, or update one when customerId is given (only the given fields " +
				"change). Check list_customers first to avoid duplicates.",
			schema: obj([]string{"workspaceId"}, merge(map[string]any{
				"workspaceId": sProp("Workspace ID"), "customerId": sProp("Existing customer to update; omit to create"),
			}, customerFields)),
			run: saveCustomer,
		},
		{
			name:        "get_quote_settings",
			description: "Issuer details and defaults used for new quotes (company, bank, number prefix, VAT, validity).",
			schema:      obj([]string{"workspaceId"}, map[string]any{"workspaceId": sProp("Workspace ID")}),
			run:         endpoint(http.MethodGet, "/v1/workspaces/{workspaceId}/quote-settings", nil, 0, nil),
		},
		{
			name:        "update_quote_settings",
			description: "Change the quote settings. Only the given fields change.",
			schema:      obj([]string{"workspaceId"}, merge(map[string]any{"workspaceId": sProp("Workspace ID")}, quoteSettingFields)),
			run:         updateQuoteSettings,
		},
	}
}

func merge(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// quoteEditable lists the fields of UpdateQuoteRequest.
var quoteEditable = []string{"title", "currency", "language", "issueDate", "validUntil", "discountType",
	"discountPercent", "discountAmount", "vatRate", "notes", "terms"}

func hasAny(a args, keys []string) bool {
	for _, k := range keys {
		if a.has(k) {
			return true
		}
	}
	return false
}

func createQuote(c *caller, a args) (any, error) {
	ws, err := a.need("workspaceId")
	if err != nil {
		return nil, err
	}
	mod, err := a.need("moduleId")
	if err != nil {
		return nil, err
	}
	body := map[string]any{"moduleId": mod}
	if v := a.str("customerId"); v != "" {
		body["customerId"] = v
	}
	created, err := c.do(http.MethodPost, "/v1/workspaces/"+p(ws)+"/quotes", nil, body)
	if err != nil {
		return nil, err
	}
	if !hasAny(a, append([]string{"items"}, quoteEditable...)) {
		return created, nil
	}
	var q struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created, &q); err != nil || q.ID == "" {
		return created, nil
	}
	a["quoteId"] = q.ID
	updated, err := updateQuote(c, a)
	if err != nil {
		return map[string]any{"quote": created, "fillError": err.Error()}, nil
	}
	return updated, nil
}

// updateQuote merges the given fields into the current draft, since the API replaces the whole quote.
func updateQuote(c *caller, a args) (any, error) {
	ws, err := a.need("workspaceId")
	if err != nil {
		return nil, err
	}
	id, err := a.need("quoteId")
	if err != nil {
		return nil, err
	}
	if !hasAny(a, append([]string{"items", "customerId"}, quoteEditable...)) {
		return nil, errors.New("nothing to update: pass at least one field")
	}
	path := "/v1/workspaces/" + p(ws) + "/quotes/" + p(id)
	raw, err := c.do(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var cur map[string]any
	if err := json.Unmarshal(raw, &cur); err != nil {
		return nil, err
	}
	if e, _ := cur["editable"].(bool); !e {
		return nil, fmt.Errorf("quote is %v and locked; use new_quote_version to edit it", cur["status"])
	}
	body := map[string]any{}
	for _, k := range quoteEditable {
		body[k] = cur[k]
	}
	if v, ok := cur["customerId"].(string); ok && v != "" {
		body["customerId"] = v
	}
	items := []any{}
	if list, ok := cur["items"].([]any); ok {
		for _, x := range list {
			if m, ok := x.(map[string]any); ok {
				items = append(items, args(m).fields(map[string]any{}, "name", "description", "unit", "quantity", "unitPrice", "discountPercent"))
			}
		}
	}
	body["items"] = items
	a.fields(body, quoteEditable...)
	if a.has("customerId") {
		body["customerId"] = a.str("customerId")
	}
	if a.has("items") {
		list, ok := a["items"].([]any)
		if !ok {
			return nil, errors.New("items must be an array")
		}
		items := make([]any, 0, len(list))
		for i, x := range list {
			m, ok := x.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("items[%d] must be an object", i)
			}
			it := args(m).fields(map[string]any{"quantity": 1.0}, "name", "description", "unit", "quantity", "unitPrice", "discountPercent")
			items = append(items, it)
		}
		body["items"] = items
	}
	return c.do(http.MethodPut, path, nil, body)
}

func exportQuote(c *caller, a args) (any, error) {
	ws, err := a.need("workspaceId")
	if err != nil {
		return nil, err
	}
	id, err := a.need("quoteId")
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	for _, k := range []string{"template", "sign", "place", "signer"} {
		if v := a.str(k); v != "" {
			q.Set(k, v)
		}
	}
	if b, _ := a["merge"].(bool); b {
		q.Set("merge", "1")
	}
	base := "/v1/workspaces/" + p(ws) + "/quotes/" + p(id)
	if a.str("format") == "xlsx" {
		return c.file(base+"/export", q, "quote.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	}
	return c.file(base+"/pdf", q, "quote.pdf", "application/pdf")
}

func saveCustomer(c *caller, a args) (any, error) {
	ws, err := a.need("workspaceId")
	if err != nil {
		return nil, err
	}
	keys := []string{"name", "taxCode", "address", "contactName", "email", "phone", "note"}
	base := "/v1/workspaces/" + p(ws) + "/customers"
	id := a.str("customerId")
	if id == "" {
		if _, err := a.need("name"); err != nil {
			return nil, err
		}
		return c.do(http.MethodPost, base, nil, a.fields(map[string]any{}, keys...))
	}
	raw, err := c.do(http.MethodGet, base, nil, nil)
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	body := map[string]any{}
	for _, m := range list {
		if m["id"] == id {
			args(m).fields(body, keys...)
		}
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("customer %s not found in this workspace", id)
	}
	return c.do(http.MethodPut, base+"/"+p(id), nil, a.fields(body, keys...))
}

func updateQuoteSettings(c *caller, a args) (any, error) {
	ws, err := a.need("workspaceId")
	if err != nil {
		return nil, err
	}
	path := "/v1/workspaces/" + p(ws) + "/quote-settings"
	raw, err := c.do(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var cur map[string]any
	if err := json.Unmarshal(raw, &cur); err != nil {
		return nil, err
	}
	keys := []string{"companyName", "address", "taxCode", "phone", "email", "website", "bankInfo", "logoUrl",
		"signatureUrl", "numberPrefix", "defaultCurrency", "defaultVatRate", "defaultValidDays", "defaultTerms"}
	if !hasAny(a, keys) {
		return nil, errors.New("nothing to update: pass at least one field")
	}
	body := args(cur).fields(map[string]any{}, keys...)
	return c.do(http.MethodPut, path, nil, a.fields(body, keys...))
}
