package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

type EntityField struct {
	Name       string
	GoType     string
	ProtoType  string
	JsonTag    string
	DbTag      string
	IsOptional bool
	SqlType    string
}

type TemplateData struct {
	EntityName       string
	EntityNameLower  string
	EntityNameSnake  string
	ProjectName      string
	ProjectNameLower string
	ProjectNameSnake string
	ProjectNameKebab string
	TableName        string
	PackageName      string
	Fields           []EntityField
	NonIdFields      []EntityField
	ProtoPackage     string
}

func main() {
	var entityName, projectName string
	flag.StringVar(&entityName, "name", "", "Entity name (e.g., Post)")
	flag.StringVar(&projectName, "project-name", "", "Project name (e.g., Blog)")
	flag.Parse()

	if entityName == "" {
		log.Fatal("Entity name is required. Use -name flag")
	}

	if projectName == "" {
		log.Fatal("Project name is required. Use -project-name flag")
	}

	if !unicode.IsUpper(rune(entityName[0])) {
		log.Fatal("Entity name must start with an uppercase letter")
	}

	if !unicode.IsUpper(rune(projectName[0])) {
		log.Fatal("Project name must start with an uppercase letter")
	}

	gen := &Generator{
		EntityName:  entityName,
		ProjectName: projectName,
	}

	if err := gen.Generate(); err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	fmt.Printf("✓ Successfully generated CRUD for entity: %s\n", entityName)
}

type Generator struct {
	EntityName  string
	ProjectName string
}

func (g *Generator) Generate() error {
	data := g.prepareTemplateData()

	if err := g.createDirectories(data); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	if err := g.generateProtoFile(data); err != nil {
		return fmt.Errorf("failed to generate proto file: %w", err)
	}

	if err := g.generateDomainFiles(data); err != nil {
		return fmt.Errorf("failed to generate domain files: %w", err)
	}

	if err := g.generateUsecaseFiles(data); err != nil {
		return fmt.Errorf("failed to generate usecase files: %w", err)
	}

	if err := g.generateHandlerFiles(data); err != nil {
		return fmt.Errorf("failed to generate handler files: %w", err)
	}

	if err := g.generateDTOFiles(data); err != nil {
		return fmt.Errorf("failed to generate DTO files: %w", err)
	}

	fmt.Println("\n⚠️  Manual steps required:")
	fmt.Println("1. Create database migration for table:", data.TableName)
	fmt.Println("2. Run: make generate")
	fmt.Println("3. Register generated code in internal/app/app.go")
	fmt.Printf("   - Import domain%sP \"github.com/mechta-market/%s/internal/domain/%s\"\n", data.EntityName, data.ProjectNameSnake, data.EntityNameSnake)
	fmt.Printf("   - Import domain%sRepoDbP \"github.com/mechta-market/%s/internal/domain/%s/repo/pg\"\n", data.EntityName, data.ProjectNameSnake, data.EntityNameSnake)
	fmt.Printf("   - Import usecase%sP \"github.com/mechta-market/%s/internal/usecase/%s\"\n", data.EntityName, data.ProjectNameSnake, data.EntityNameSnake)
	fmt.Printf("   - Add variables and initialization\n")
	fmt.Printf("   - Register in grpc server and gateway\n")

	return nil
}

func (g *Generator) prepareTemplateData() *TemplateData {
	entityNameLower := strings.ToLower(string(g.EntityName[0])) + g.EntityName[1:]
	entityNameSnake := toSnakeCase(g.EntityName)
	projectNameLower := strings.ToLower(string(g.ProjectName[0])) + g.ProjectName[1:]
	projectNameSnake := toSnakeCase(g.ProjectName)
	projectNameKebab := toKebabCase(g.ProjectName)

	fields := []EntityField{
		{Name: "Id", GoType: "int64", ProtoType: "int64", JsonTag: "id", DbTag: "id", IsOptional: false, SqlType: "BIGSERIAL PRIMARY KEY"},
		{Name: "Title", GoType: "string", ProtoType: "string", JsonTag: "title", DbTag: "title", IsOptional: false, SqlType: "VARCHAR(255) NOT NULL"},
		{Name: "Description", GoType: "string", ProtoType: "string", JsonTag: "description", DbTag: "description", IsOptional: true, SqlType: "TEXT"},
		{Name: "CreatedAt", GoType: "time.Time", ProtoType: "string", JsonTag: "created_at", DbTag: "created_at", IsOptional: false, SqlType: "TIMESTAMP NOT NULL DEFAULT NOW()"},
		{Name: "UpdatedAt", GoType: "time.Time", ProtoType: "string", JsonTag: "updated_at", DbTag: "updated_at", IsOptional: false, SqlType: "TIMESTAMP NOT NULL DEFAULT NOW()"},
	}

	nonIdFields := []EntityField{}
	for _, f := range fields {
		if f.Name != "Id" {
			nonIdFields = append(nonIdFields, f)
		}
	}

	return &TemplateData{
		EntityName:       g.EntityName,
		EntityNameLower:  entityNameLower,
		EntityNameSnake:  entityNameSnake,
		ProjectName:      g.ProjectName,
		ProjectNameLower: projectNameLower,
		ProjectNameSnake: projectNameSnake,
		ProjectNameKebab: projectNameKebab,
		TableName:        entityNameSnake + "s",
		PackageName:      entityNameSnake,
		Fields:           fields,
		NonIdFields:      nonIdFields,
		ProtoPackage:     fmt.Sprintf("%s_v1", projectNameSnake),
	}
}

func (g *Generator) createDirectories(data *TemplateData) error {
	dirs := []string{
		filepath.Join("api", "proto", fmt.Sprintf("%s_v1", data.ProjectNameSnake)),
		filepath.Join("internal", "domain", data.EntityNameSnake),
		filepath.Join("internal", "domain", data.EntityNameSnake, "model"),
		filepath.Join("internal", "domain", data.EntityNameSnake, "repo", "pg"),
		filepath.Join("internal", "domain", data.EntityNameSnake, "repo", "pg", "model"),
		filepath.Join("internal", "usecase", data.EntityNameSnake),
		filepath.Join("internal", "handler", "grpc"),
		filepath.Join("internal", "handler", "grpc", "dto"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (g *Generator) generateProtoFile(data *TemplateData) error {
	tmpl := `syntax = "proto3";

package {{.ProjectNameSnake}};

import "google/api/annotations.proto";
import "google/protobuf/empty.proto";
import "common/common.proto";

option go_package = "/{{.ProjectNameSnake}}_v1";

service {{.EntityName}} {
  rpc Create({{.EntityName}}CreateReq) returns ({{.EntityName}}CreateRep) {
    option (google.api.http) = {
      post: "/{{.EntityNameSnake}}"
      body: "*"
    };
  }

  rpc GetById({{.EntityName}}GetByIdReq) returns ({{.EntityName}}Main) {
    option (google.api.http) = {
      get: "/{{.EntityNameSnake}}/{id}"
    };
  }

  rpc List({{.EntityName}}ListReq) returns ({{.EntityName}}ListRep) {
    option (google.api.http) = {
      get: "/{{.EntityNameSnake}}"
    };
  }

  rpc Update({{.EntityName}}UpdateReq) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      put: "/{{.EntityNameSnake}}/{id}"
      body: "*"
    };
  }

  rpc Delete({{.EntityName}}DeleteReq) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      delete: "/{{.EntityNameSnake}}/{id}"
    };
  }
}

message {{.EntityName}}Main {
{{range $i, $f := .Fields}}  {{if $f.IsOptional}}optional {{end}}{{$f.ProtoType}} {{$f.JsonTag}} = {{add $i 1}};
{{end}}}

message {{.EntityName}}CreateReq {
{{range $i, $f := .NonIdFields}}{{if ne $f.Name "CreatedAt"}}{{if ne $f.Name "UpdatedAt"}}  {{if $f.IsOptional}}optional {{end}}{{$f.ProtoType}} {{$f.JsonTag}} = {{add $i 1}};
{{end}}{{end}}{{end}}}

message {{.EntityName}}CreateRep {
  int64 id = 1;
}

message {{.EntityName}}GetByIdReq {
  int64 id = 1;
}

message {{.EntityName}}ListReq {
  common.ListParamsSt list_params = 1;
  optional string title = 2;
  optional string created_at_from = 3;
  optional string created_at_to = 4;
}

message {{.EntityName}}ListRep {
  common.PaginationInfoSt pagination_info = 1;
  repeated {{.EntityName}}Main results = 2;
}

message {{.EntityName}}UpdateReq {
  int64 id = 1;
{{range $i, $f := .NonIdFields}}{{if ne $f.Name "CreatedAt"}}{{if ne $f.Name "UpdatedAt"}}  {{if $f.IsOptional}}optional {{end}}{{$f.ProtoType}} {{$f.JsonTag}} = {{add $i 2}};
{{end}}{{end}}{{end}}}

message {{.EntityName}}DeleteReq {
  int64 id = 1;
}
`

	return g.executeTemplate(tmpl, data, filepath.Join("api", "proto", fmt.Sprintf("%s_v1", data.ProjectNameSnake), data.EntityNameSnake+".proto"))
}

func (g *Generator) generateDomainFiles(data *TemplateData) error {
	// model.go
	modelTmpl := `package model

import (
	"time"

	commonModel "github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/common/model"
)

type Main struct {
{{range .Fields}}	{{.Name}} {{if .IsOptional}}*{{end}}{{.GoType}}
{{end}}}

type Edit struct {
{{range .Fields}}	{{.Name}} *{{.GoType}}
{{end}}}

type ListReq struct {
	commonModel.ListParams

	Title         *string
	CreatedAtFrom *time.Time
	CreatedAtTo   *time.Time
}
`
	if err := g.executeTemplate(modelTmpl, data, filepath.Join("internal", "domain", data.EntityNameSnake, "model", "model.go")); err != nil {
		return err
	}

	// interface.go
	interfaceTmpl := `package {{.PackageName}}

import (
	"context"

	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/{{.EntityNameSnake}}/model"
)

type RepoI interface {
	Create(ctx context.Context, m *model.Edit) (int64, error)
	GetById(ctx context.Context, id int64) (*model.Main, bool, error)
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Update(ctx context.Context, m *model.Edit) error
	Delete(ctx context.Context, id int64) error
}
`
	if err := g.executeTemplate(interfaceTmpl, data, filepath.Join("internal", "domain", data.EntityNameSnake, "interface.go")); err != nil {
		return err
	}

	// service.go
	serviceTmpl := `package {{.PackageName}}

import (
	"context"

	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/{{.EntityNameSnake}}/model"
)

type Service struct {
	repo RepoI
}

func New(repo RepoI) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) Create(ctx context.Context, m *model.Edit) (int64, error) {
	id, err := s.repo.Create(ctx, m)
    
    if err != nil {return 0, err}

    return id, nil
}

func (s *Service) GetById(ctx context.Context, id int64, errNE bool) (*model.Main, bool, error) {
	model, found, err := s.repo.GetById(ctx, id)
    
    if err != nil {
		return nil, false, fmt.Errorf("service.GetById: %w", err)
	}
    
	if !found {
		if errNE {
			return nil, false, errs.ObjectNotFound
		}
		return nil, false, nil
	}

    return model, true, nil
}

func (s *Service) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	models, tCount, err := s.repo.List(ctx, pars)
   
   if err != nil { return nil, 0, fmt.Errorf("service.List: %w", err)}

   return models, tCount, nil
}

func (s *Service) Update(ctx context.Context, m *model.Edit) error {
	err := s.repo.Update(ctx, m)
    
    if err != nil { return fmt.Errorf("service.Update: %w", err)}
    
    return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	err := s.repo.Delete(ctx, id)
    
    if err != nil { return fmt.Errorf("service.Delete: %w", err)}

	return nil
}
`
	if err := g.executeTemplate(serviceTmpl, data, filepath.Join("internal", "domain", data.EntityNameSnake, data.EntityNameSnake+".go")); err != nil {
		return err
	}

	// repo/pg/pg.go
	repoTmpl := `package pg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	commonRepoPg "github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/common/repo/pg"
	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/{{.EntityNameSnake}}/model"
	repoModel "github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/{{.EntityNameSnake}}/repo/pg/model"
	"github.com/mechta-market/mobone/v2"
	"github.com/opentracing/opentracing-go"
	"github.com/samber/lo"
)

type Repo struct {
	*commonRepoPg.Base
	ModelStore *mobone.ModelStore
}

func New(con *pgxpool.Pool) *Repo {
	base := commonRepoPg.NewBase(con)
	return &Repo{
		Base: base,
		ModelStore: &mobone.ModelStore{
			Con:       base.Con,
			QB:        base.QB,
			TableName: "{{.TableName}}",
		},
	}
}

func (r *Repo) Create(ctx context.Context, m *model.Edit) (_ int64, finalError error) {
	tracingSpan, ctx := opentracing.StartSpanFromContext(ctx, "{{.PackageName}}.repo.PG.Create")
	defer tracingSpan.Finish()
	defer func() {
		if finalError != nil {
			tracingSpan.SetTag("error", true)
			tracingSpan.LogKV("error", finalError.Error())
		}
	}()

	upsertObj := repoModel.EncodeEdit(m)

	err := r.ModelStore.Create(ctx, upsertObj)
	if err != nil {
		return 0, fmt.Errorf("ModelStore.Create: %w", err)
	}

	if upsertObj.Id == nil {
		return 0, fmt.Errorf("no id returned on create {{.EntityNameSnake}}")
	}

	return *upsertObj.Id, nil
}

func (r *Repo) GetById(ctx context.Context, id int64) (_ *model.Main, _ bool, finalError error) {
	tracingSpan, ctx := opentracing.StartSpanFromContext(ctx, "{{.PackageName}}.repo.PG.GetById")
	defer tracingSpan.Finish()
	defer func() {
		if finalError != nil {
			tracingSpan.SetTag("error", true)
			tracingSpan.LogKV("error", finalError.Error())
		}
	}()

	m := &repoModel.Select{
		Id: id,
	}
	found, err := r.ModelStore.Get(ctx, m)
	if err != nil {
		return nil, false, fmt.Errorf("ModelStore.Get: %w", err)
	}
	if !found {
		return nil, false, nil
	}

	return repoModel.DecodeMain(m, 0), true, nil
}

func (r *Repo) List(ctx context.Context, pars *model.ListReq) (_ []*model.Main, _ int64, finalError error) {
	tracingSpan, ctx := opentracing.StartSpanFromContext(ctx, "{{.PackageName}}.repo.PG.List")
	defer tracingSpan.Finish()
	defer func() {
		if finalError != nil {
			tracingSpan.SetTag("error", true)
			tracingSpan.LogKV("error", finalError.Error())
		}
	}()

	conditions, conditionExps := r.getConditions(pars)

	items := make([]*repoModel.Select, 0)

	totalCount, err := r.ModelStore.List(ctx, mobone.ListParams{
		Conditions:           conditions,
		ConditionExpressions: conditionExps,
		Page:                 pars.Page,
		PageSize:             pars.PageSize,
		WithTotalCount:       pars.WithTotalCount,
		OnlyCount:            pars.OnlyCount,
		Sort:                 pars.Sort,
	}, func(add bool) mobone.ListModelI {
		item := &repoModel.Select{}
		if add {
			items = append(items, item)
		}
		return item
	})
	if err != nil {
		return nil, 0, fmt.Errorf("ModelStore.List: %w", err)
	}

	return lo.Map(items, repoModel.DecodeMain), totalCount, nil
}

func (r *Repo) Update(ctx context.Context, m *model.Edit) (finalError error) {
	tracingSpan, ctx := opentracing.StartSpanFromContext(ctx, "{{.PackageName}}.repo.PG.Update")
	defer tracingSpan.Finish()
	defer func() {
		if finalError != nil {
			tracingSpan.SetTag("error", true)
			tracingSpan.LogKV("error", finalError.Error())
		}
	}()

	upsertObj := repoModel.EncodeEdit(m)

	err := r.ModelStore.Update(ctx, upsertObj)
	if err != nil {
		return fmt.Errorf("ModelStore.Update: %w", err)
	}

	return nil
}

func (r *Repo) Delete(ctx context.Context, id int64) (finalError error) {
	tracingSpan, ctx := opentracing.StartSpanFromContext(ctx, "{{.PackageName}}.repo.PG.Delete")
	defer tracingSpan.Finish()
	defer func() {
		if finalError != nil {
			tracingSpan.SetTag("error", true)
			tracingSpan.LogKV("error", finalError.Error())
		}
	}()

	upsertObj := &repoModel.Upsert{
		Id: &id,
	}

	err := r.ModelStore.Delete(ctx, upsertObj)
	if err != nil {
		return fmt.Errorf("ModelStore.Delete: %w", err)
	}

	return nil
}
`
	if err := g.executeTemplate(repoTmpl, data, filepath.Join("internal", "domain", data.EntityNameSnake, "repo", "pg", "pg.go")); err != nil {
		return err
	}

	// repo/pg/custom.go
	customTmpl := `package pg

import (
	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/{{.EntityNameSnake}}/model"
)

func (r *Repo) getConditions(pars *model.ListReq) (map[string]any, map[string][]any) {
	conditions := make(map[string]any)
	conditionExps := make(map[string][]any)

	// Filter by title using ILIKE (case-insensitive LIKE)
	if pars.Title != nil && *pars.Title != "" {
		conditionExps["title ILIKE ?"] = []any{"%" + *pars.Title + "%"}
	}

	// Filter by created_at date range
	if pars.CreatedAtFrom != nil {
		conditionExps["created_at >= ?"] = []any{*pars.CreatedAtFrom}
	}
	if pars.CreatedAtTo != nil {
		conditionExps["created_at <= ?"] = []any{*pars.CreatedAtTo}
	}

	return conditions, conditionExps
}
`
	if err := g.executeTemplate(customTmpl, data, filepath.Join("internal", "domain", data.EntityNameSnake, "repo", "pg", "custom.go")); err != nil {
		return err
	}

	// repo/pg/model/select.go
	selectTmpl := `package model

import (
	"database/sql"
)

type Select struct {
{{range .Fields}}	{{.Name}} {{if .IsOptional}}sql.Null{{if eq .GoType "string"}}String{{else if eq .GoType "time.Time"}}Time{{else if eq .GoType "int64"}}Int64{{end}}{{else}}{{.GoType}}{{end}}
{{end}}}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
{{range .Fields}}		"{{.DbTag}}": &m.{{.Name}},
{{end}}	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{
		"id": m.Id,
	}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{"id desc"}
}
`
	if err := g.executeTemplate(selectTmpl, data, filepath.Join("internal", "domain", data.EntityNameSnake, "repo", "pg", "model", "select.go")); err != nil {
		return err
	}

	// repo/pg/model/upsert.go
	upsertTmpl := `package model

import "time"

type Upsert struct {
{{range .Fields}}	{{.Name}} *{{.GoType}}
{{end}}}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := make(map[string]any)

{{range .NonIdFields}}	if m.{{.Name}} != nil {
		result["{{.DbTag}}"] = *m.{{.Name}}
	}
{{end}}
	return result
}

func (m *Upsert) UpdateColumnMap() map[string]any {
	res := m.CreateColumnMap()
	pkMap := m.PKColumnMap()
	for k := range pkMap {
		delete(res, k)
	}
	return res
}

func (m *Upsert) PKColumnMap() map[string]any {
	return map[string]any{
		"id": m.Id,
	}
}

func (m *Upsert) ReturningColumnMap() map[string]any {
	return map[string]any{
		"id": &m.Id,
	}
}
`
	if err := g.executeTemplate(upsertTmpl, data, filepath.Join("internal", "domain", data.EntityNameSnake, "repo", "pg", "model", "upsert.go")); err != nil {
		return err
	}

	// repo/pg/model/dto.go
	dtoTmpl := `package model

import (
	"time"

	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/{{.EntityNameSnake}}/model"
)

func EncodeEdit(m *model.Edit) *Upsert {
	return &Upsert{
{{range .Fields}}		{{.Name}}: m.{{.Name}},
{{end}}	}
}

func DecodeMain(m *Select, _ int) *model.Main {
{{range .NonIdFields}}{{if .IsOptional}}	var {{.JsonTag}} {{.GoType}}
	if m.{{.Name}}.Valid {
		{{if eq .GoType "string"}}{{.JsonTag}} = m.{{.Name}}.String{{else if eq .GoType "time.Time"}}{{.JsonTag}} = m.{{.Name}}.Time{{else if eq .GoType "int64"}}{{.JsonTag}} = m.{{.Name}}.Int64{{end}}
	}
{{end}}{{end}}
{{range .NonIdFields}}{{if not .IsOptional}}	var {{.JsonTag}} {{.GoType}}
	if m.{{.Name}}.Valid {
		{{if eq .GoType "string"}}{{.JsonTag}} = m.{{.Name}}.String{{else if eq .GoType "time.Time"}}{{.JsonTag}} = m.{{.Name}}.Time{{else if eq .GoType "int64"}}{{.JsonTag}} = m.{{.Name}}.Int64{{end}}
	}
{{end}}{{end}}
	return &model.Main{
		Id: m.Id,
{{range .NonIdFields}}		{{.Name}}: {{if .IsOptional}}&{{end}}{{.JsonTag}},
{{end}}	}
}
`
	return g.executeTemplate(dtoTmpl, data, filepath.Join("internal", "domain", data.EntityNameSnake, "repo", "pg", "model", "dto.go"))
}

func (g *Generator) generateUsecaseFiles(data *TemplateData) error {
	interfaceTmpl := `package {{.PackageName}}

import (
	"context"

	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/{{.EntityNameSnake}}/model"
)

type {{.EntityName}}ServiceI interface {
	Create(ctx context.Context, m *model.Edit) (int64, error)
	GetById(ctx context.Context, id int64, errNE bool) (*model.Main, bool, error)
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Update(ctx context.Context, m *model.Edit) error
	Delete(ctx context.Context, id int64) error
}
`
	if err := g.executeTemplate(interfaceTmpl, data, filepath.Join("internal", "usecase", data.EntityNameSnake, "interfaces.go")); err != nil {
		return err
	}

	usecaseTmpl := `package {{.PackageName}}

import (
	"context"
	"fmt"

	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/constant"
	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/common/util"
	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/{{.EntityNameSnake}}/model"
	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/errs"
)

type Usecase struct {
	{{.EntityNameLower}}Service {{.EntityName}}ServiceI
}

func New({{.EntityNameLower}}Service {{.EntityName}}ServiceI) *Usecase {
	return &Usecase{
		{{.EntityNameLower}}Service: {{.EntityNameLower}}Service,
	}
}

func (u *Usecase) Create(ctx context.Context, m *model.Edit) (int64, error) {
	id, err := u.{{.EntityNameLower}}Service.Create(ctx, m)
	if err != nil {
		return 0, fmt.Errorf("{{.EntityNameLower}}Service.Create: %w", err)
	}

	return id, nil
}

func (u *Usecase) GetById(ctx context.Context, id int64) (*model.Main, error) {
	result, _, err := u.{{.EntityNameLower}}Service.GetById(ctx, id, true)

	if err != nil {
		return nil, fmt.Errorf("{{.EntityNameLower}}Service.GetById: %w", err)
	}

	return result, nil
}

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	if err := util.RequirePageSize(pars.ListParams, constant.MaxPageSize); err != nil {
		return nil, 0, errs.IncorrectPageSize
	}

	items, totalCount, err := u.{{.EntityNameLower}}Service.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("{{.EntityNameLower}}Service.List: %w", err)
	}

	return items, totalCount, nil
}

func (u *Usecase) Update(ctx context.Context, m *model.Edit) error {
	if m.Id == nil {
		return errs.InvalidInput
	}

	err := u.{{.EntityNameLower}}Service.Update(ctx, m)
	if err != nil {
		return fmt.Errorf("{{.EntityNameLower}}Service.Update: %w", err)
	}

	return nil
}

func (u *Usecase) Delete(ctx context.Context, id int64) error {
	err := u.{{.EntityNameLower}}Service.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("{{.EntityNameLower}}Service.Delete: %w", err)
	}

	return nil
}
`
	return g.executeTemplate(usecaseTmpl, data, filepath.Join("internal", "usecase", data.EntityNameSnake, "usecase.go"))
}

func (g *Generator) generateHandlerFiles(data *TemplateData) error {
	handlerTmpl := `package grpc

import (
	"context"
	"fmt"

	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/{{.EntityNameSnake}}/model"
	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/handler/grpc/dto"
	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/usecase/{{.EntityNameSnake}}"
	"github.com/mechta-market/{{.ProjectNameKebab}}/pkg/proto/{{.ProjectNameSnake}}_v1"
	"github.com/mechta-market/{{.ProjectNameKebab}}/pkg/proto/common"
	"google.golang.org/protobuf/types/known/emptypb"
)

type {{.EntityName}} struct {
	{{.ProjectNameSnake}}_v1.Unsafe{{.EntityName}}Server
	usecase *{{.PackageName}}.Usecase
}

func New{{.EntityName}}(usecase *{{.PackageName}}.Usecase) *{{.EntityName}} {
	return &{{.EntityName}}{
		usecase: usecase,
	}
}

func (h *{{.EntityName}}) Create(ctx context.Context, req *{{.ProjectNameSnake}}_v1.{{.EntityName}}CreateReq) (*{{.ProjectNameSnake}}_v1.{{.EntityName}}CreateRep, error) {
	if err != nil {
		return nil, err
	}

	id, err := h.usecase.Create(ctx, dto.Decode{{.EntityName}}CreateReq(req))
	if err != nil {
		return nil, fmt.Errorf("usecase.Create: %w", err)
	}

	return &{{.ProjectNameSnake}}_v1.{{.EntityName}}CreateRep{
		Id: id,
	}, nil
}

func (h *{{.EntityName}}) GetById(ctx context.Context, req *{{.ProjectNameSnake}}_v1.{{.EntityName}}GetByIdReq) (*{{.ProjectNameSnake}}_v1.{{.EntityName}}Main, error) {
	if err != nil {
		return nil, err
	}

	result, err := h.usecase.GetById(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("usecase.GetById: %w", err)
	}

	return dto.Encode{{.EntityName}}Main(result), nil
}

func (h *{{.EntityName}}) List(ctx context.Context, req *{{.ProjectNameSnake}}_v1.{{.EntityName}}ListReq) (*{{.ProjectNameSnake}}_v1.{{.EntityName}}ListRep, error) {
	if err != nil {
		return nil, err
	}

	items, tCount, err := h.usecase.List(ctx, dto.Decode{{.EntityName}}ListReq(req))
	if err != nil {
		return nil, fmt.Errorf("usecase.List: %w", err)
	}

	resultItems := make([]*{{.ProjectNameSnake}}_v1.{{.EntityName}}Main, len(items))
	for i, item := range items {
		resultItems[i] = dto.Encode{{.EntityName}}Main(item)
	}

	return &{{.ProjectNameSnake}}_v1.{{.EntityName}}ListRep{
		PaginationInfo: &common.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: resultItems,
	}, nil
}

func (h *{{.EntityName}}) Update(ctx context.Context, req *{{.ProjectNameSnake}}_v1.{{.EntityName}}UpdateReq) (*emptypb.Empty, error) {
	if err != nil {
		return nil, err
	}

	err = h.usecase.Update(ctx, dto.Decode{{.EntityName}}UpdateReq(req))
	if err != nil {
		return nil, fmt.Errorf("usecase.Update: %w", err)
	}

	return &emptypb.Empty{}, nil
}

func (h *{{.EntityName}}) Delete(ctx context.Context, req *{{.ProjectNameSnake}}_v1.{{.EntityName}}DeleteReq) (*emptypb.Empty, error) {
	if err != nil {
		return nil, err
	}

	err = h.usecase.Delete(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("usecase.Delete: %w", err)
	}

	return &emptypb.Empty{}, nil
}
`
	return g.executeTemplate(handlerTmpl, data, filepath.Join("internal", "handler", "grpc", data.EntityNameSnake+".go"))
}

func (g *Generator) generateDTOFiles(data *TemplateData) error {
	dtoTmpl := `package dto

import (
	"time"

	"github.com/mechta-market/{{.ProjectNameKebab}}/internal/domain/{{.EntityNameSnake}}/model"
	"github.com/mechta-market/{{.ProjectNameKebab}}/pkg/proto/{{.ProjectNameSnake}}_v1"
)

func Encode{{.EntityName}}Main(m *model.Main) *{{.ProjectNameSnake}}_v1.{{.EntityName}}Main {
	if m == nil {
		return nil
	}

	return &{{.ProjectNameSnake}}_v1.{{.EntityName}}Main{
		Id:          m.Id,
		Title:       m.Title,
		Description: m.Description,
		CreatedAt:   m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   m.UpdatedAt.Format(time.RFC3339),
	}
}

func Decode{{.EntityName}}CreateReq(req *{{.ProjectNameSnake}}_v1.{{.EntityName}}CreateReq) *model.Edit {
	if req == nil {
		return nil
	}

	return &model.Edit{
		Title:       &req.Title,
		Description: &req.Description,
	}
}

func Decode{{.EntityName}}UpdateReq(req *{{.ProjectNameSnake}}_v1.{{.EntityName}}UpdateReq) *model.Edit {
	if req == nil {
		return nil
	}

	return &model.Edit{
		Id:          &req.Id,
		Title:       &req.Title,
		Description: &req.Description,
	}
}

func Decode{{.EntityName}}ListReq(req *{{.ProjectNameSnake}}_v1.{{.EntityName}}ListReq) *model.ListReq {
	if req == nil {
		return nil
	}

	limit := req.ListParams.PageSize
	offset := (req.ListParams.Page - 1) * req.ListParams.PageSize

	result := &model.ListReq{
		ListParams: model.ListParams{
			Limit:  &limit,
			Offset: &offset,
		},
	}

	if req.Title != nil {
		result.Title = req.Title
	}

	if req.CreatedAtFrom != nil {
		t, err := time.Parse(time.RFC3339, *req.CreatedAtFrom)
		if err == nil {
			result.CreatedAtFrom = &t
		}
	}

	if req.CreatedAtTo != nil {
		t, err := time.Parse(time.RFC3339, *req.CreatedAtTo)
		if err == nil {
			result.CreatedAtTo = &t
		}
	}

	return result
}
`
	return g.executeTemplate(dtoTmpl, data, filepath.Join("internal", "handler", "grpc", "dto", data.EntityNameSnake+".go"))
}

func (g *Generator) executeTemplate(tmplStr string, data interface{}, outputPath string) error {
	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
	}

	tmpl, err := template.New("template").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", outputPath, err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

func toKebabCase(s string) string {
	var result []rune
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result = append(result, '-')
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
