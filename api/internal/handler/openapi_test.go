package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/yama-shu/575-sns/api/internal/domain"
	"github.com/yama-shu/575-sns/api/internal/handler"
)

// api/openapi.yaml は手で書いている（基本設計 05 §6）。
// prosody は FastAPI が生成するためずれないが、Go には生成元が無い。
//
// **手で書いた定義は、書いた時点から実装とずれ始める。**
// 生成物のずれ（web の型）は CI で検出できるが、
// 定義とサーバーの実際の応答のずれは検出できない。
//
// ここでは、ハンドラが実際に返す JSON が定義のスキーマに適合することを確かめる。
// 定義を直さずに応答だけ変えれば、このテストが落ちる。

var specPath = filepath.Join("..", "..", "openapi.yaml")

// loadSpec は定義を読む。
func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()

	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("定義が見つからない: %v", err)
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("定義を読めない: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("定義が妥当でない: %v", err)
	}
	return doc
}

// schemaOf は指定した操作・ステータスの JSON スキーマを返す。
//
// path は定義上のパス（`/posts/{id}` のようにテンプレートのまま）。
func schemaOf(t *testing.T, doc *openapi3.T, method, path, status string) *openapi3.Schema {
	t.Helper()

	item := doc.Paths.Find(path)
	if item == nil {
		t.Fatalf("定義にパスが無い: %s", path)
	}
	op := item.GetOperation(method)
	if op == nil {
		t.Fatalf("定義に操作が無い: %s %s", method, path)
	}
	res := op.Responses.Status(statusCode(t, status))
	if res == nil || res.Value == nil {
		t.Fatalf("定義に応答が無い: %s %s %s", method, path, status)
	}
	media := res.Value.Content.Get("application/json")
	if media == nil || media.Schema == nil {
		t.Fatalf("定義に JSON スキーマが無い: %s %s %s", method, path, status)
	}
	return media.Schema.Value
}

func statusCode(t *testing.T, status string) int {
	t.Helper()
	switch status {
	case "200":
		return http.StatusOK
	case "201":
		return http.StatusCreated
	case "400":
		return http.StatusBadRequest
	case "401":
		return http.StatusUnauthorized
	case "403":
		return http.StatusForbidden
	case "404":
		return http.StatusNotFound
	case "409":
		return http.StatusConflict
	case "422":
		return http.StatusUnprocessableEntity
	case "503":
		return http.StatusServiceUnavailable
	}
	t.Fatalf("扱っていないステータス: %s", status)
	return 0
}

// assertConforms は応答が定義のスキーマに適合することを確かめる。
func assertConforms(t *testing.T, doc *openapi3.T, method, path, status string, body map[string]any) {
	t.Helper()

	schema := schemaOf(t, doc, method, path, status)
	// 応答のスキーマには `additionalProperties: false` を付けてある。
	// **定義に無い項目を返すと落ちる。** 付けないと、実装に項目を足して
	// 定義に書き忘れても通ってしまう（この検査を入れたときに実際に通った）。
	opts := []openapi3.SchemaValidationOption{
		openapi3.VisitAsResponse(),
		openapi3.MultiErrors(),
	}
	if err := schema.VisitJSON(normalize(body), opts...); err != nil {
		t.Errorf("%s %s の %s の応答が定義に適合しない:\n%v\n応答: %v", method, path, status, err, body)
	}
}

// normalize は json.Unmarshal の結果をスキーマ検証にかけられる形にする。
func normalize(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return v
	}
	return out
}

func TestSpecIsValid(t *testing.T) {
	loadSpec(t)
}

// 定義に、実装しているエンドポイントがすべて載っていること。
//
// 実装を足したのに定義に書き忘れる、を防ぐ。
func TestSpecCoversImplementedEndpoints(t *testing.T) {
	doc := loadSpec(t)

	// cmd/api/main.go で登録している経路（/healthz と /readyz は
	// バージョン付きの経路の外にあるため対象外）。
	want := map[string][]string{
		"/auth/signup":           {http.MethodPost},
		"/auth/login":            {http.MethodPost},
		"/auth/logout":           {http.MethodPost},
		"/me":                    {http.MethodGet},
		"/me/profile":            {http.MethodPatch},
		"/prosody/check":         {http.MethodPost},
		"/posts":                 {http.MethodPost},
		"/posts/{id}":            {http.MethodGet, http.MethodDelete},
		"/posts/{id}/like":       {http.MethodPut, http.MethodDelete},
		"/posts/{id}/report":     {http.MethodPost},
		"/timelines/public":      {http.MethodGet},
		"/timelines/home":        {http.MethodGet},
		"/users/{handle}":        {http.MethodGet},
		"/users/{handle}/posts":  {http.MethodGet},
		"/users/{handle}/follow": {http.MethodPut, http.MethodDelete},
		"/users/{handle}/block":  {http.MethodPut, http.MethodDelete},
	}

	for path, methods := range want {
		item := doc.Paths.Find(path)
		if item == nil {
			t.Errorf("定義にパスが無い: %s", path)
			continue
		}
		for _, method := range methods {
			if item.GetOperation(method) == nil {
				t.Errorf("定義に操作が無い: %s %s", method, path)
			}
		}
	}
}

// 成功時の応答が定義に適合すること。
func TestResponsesConformToSpec(t *testing.T) {
	doc := loadSpec(t)

	t.Run("POST /posts の 201", func(t *testing.T) {
		_, body := callPost(t, http.MethodPost, "/api/v1/posts",
			`{"body":"今日もまた会議のための会議かな"}`,
			&stubPostRepo{}, &stubAnalyzer{result: teikeiAnalysis()}, poster(),
			(*handler.Post).Create)
		assertConforms(t, doc, http.MethodPost, "/posts", "201", body)
	})

	t.Run("GET /posts/{id} の 200", func(t *testing.T) {
		repo := &stubPostRepo{post: publishedPost(), author: poster()}
		_, body := callPost(t, http.MethodGet, "/api/v1/posts/1234", "",
			repo, &stubAnalyzer{}, nil, (*handler.Post).Get)
		assertConforms(t, doc, http.MethodGet, "/posts/{id}", "200", body)
	})

	t.Run("POST /prosody/check の 200", func(t *testing.T) {
		_, body := call(t, &stubAnalyzer{result: teikeiAnalysis()},
			`{"body":"今日もまた会議のための会議かな"}`)
		assertConforms(t, doc, http.MethodPost, "/prosody/check", "200", body)
	})

	t.Run("POST /prosody/check の 200（読めない語）", func(t *testing.T) {
		_, body := call(t, &stubAnalyzer{result: unknownAnalysis()}, `{"body":"甃"}`)
		assertConforms(t, doc, http.MethodPost, "/prosody/check", "200", body)
	})

	t.Run("GET /users/{handle} の 200", func(t *testing.T) {
		_, body := callProfile(t, "/api/v1/users/yamada", "yamada",
			usersWith(profileOwner()), &stubUserTimelineRepo{}, nil,
			(*handler.Profile).Get)
		assertConforms(t, doc, http.MethodGet, "/users/{handle}", "200", body)
	})

	t.Run("PATCH /me/profile の 200", func(t *testing.T) {
		owner := profileOwner()
		_, body := callUpdateProfile(t, `{"display_name":"やまだ改"}`, owner, usersWith(owner))
		assertConforms(t, doc, http.MethodPatch, "/me/profile", "200", body)
	})

	t.Run("GET /admin/reports の 200", func(t *testing.T) {
		repo := &stubAdminRepo{items: []domain.PendingReport{reportedItem()}}
		_, body := callAdmin(t, http.MethodGet, "/api/v1/admin/reports", "",
			repo, adminOwner(), (*handler.Admin).Reports)
		assertConforms(t, doc, http.MethodGet, "/admin/reports", "200", body)
	})

	t.Run("GET /users/{handle}/following の 200", func(t *testing.T) {
		repo := &stubRelationListRepo{items: []domain.RelationListItem{listedUser()}}
		_, body := callRelationList(t, "/api/v1/users/yamada/following", "yamada",
			repo, usersWith(profileOwner()), nil, (*handler.RelationList).Following)
		assertConforms(t, doc, http.MethodGet, "/users/{handle}/following", "200", body)
	})

	t.Run("GET /me/blocks の 200", func(t *testing.T) {
		repo := &stubRelationListRepo{items: []domain.RelationListItem{listedUser()}}
		_, body := callRelationList(t, "/api/v1/me/blocks", "",
			repo, usersWith(profileOwner()), profileOwner(), (*handler.RelationList).Blocking)
		assertConforms(t, doc, http.MethodGet, "/me/blocks", "200", body)
	})

	t.Run("GET /users/{handle}/posts の 200", func(t *testing.T) {
		timelines := &stubUserTimelineRepo{items: []domain.TimelineItem{
			{Post: publishedPost(), Author: profileOwner()},
		}}
		_, body := callProfile(t, "/api/v1/users/yamada/posts", "yamada",
			usersWith(profileOwner()), timelines, nil,
			(*handler.Profile).Posts)
		assertConforms(t, doc, http.MethodGet, "/users/{handle}/posts", "200", body)
	})
}

// エラー応答が定義に適合すること。
//
// **`code` で分岐する契約**（基本設計 05 §1）が守られているかを見る。
func TestErrorResponsesConformToSpec(t *testing.T) {
	doc := loadSpec(t)

	t.Run("400 VALIDATION_FAILED", func(t *testing.T) {
		_, body := call(t, &stubAnalyzer{}, `{"body":""}`)
		assertConforms(t, doc, http.MethodPost, "/prosody/check", "400", body)
	})

	t.Run("401 UNAUTHENTICATED", func(t *testing.T) {
		_, body := callPost(t, http.MethodPost, "/api/v1/posts", `{"body":"本文"}`,
			&stubPostRepo{}, &stubAnalyzer{}, nil, (*handler.Post).Create)
		assertConforms(t, doc, http.MethodPost, "/posts", "401", body)
	})

	t.Run("422 PROSODY_HACHO", func(t *testing.T) {
		_, body := callPost(t, http.MethodPost, "/api/v1/posts", `{"body":"今日は疲れた"}`,
			&stubPostRepo{}, &stubAnalyzer{result: hachoAnalysis()}, poster(),
			(*handler.Post).Create)
		assertConforms(t, doc, http.MethodPost, "/posts", "422", body)
	})

	t.Run("503 PROSODY_UNAVAILABLE", func(t *testing.T) {
		_, body := callPost(t, http.MethodPost, "/api/v1/posts", `{"body":"本文"}`,
			&stubPostRepo{}, &stubAnalyzer{err: domain.ErrProsodyUnavailable}, poster(),
			(*handler.Post).Create)
		assertConforms(t, doc, http.MethodPost, "/posts", "503", body)
	})
}
