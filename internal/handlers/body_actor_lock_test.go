package handlers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// actorFieldPattern - имена полей тела, похожие на «кто сделал»: автор, исполнитель,
// владелец, получатель. Такое поле из тела либо игнорируется (действующее лицо берётся из
// токена), либо означает другого человека (адресат пересылки, назначаемый принимающий).
// «<причастие>_by» ловит created_by, banned_by, approvedby и не трогает sort_by/order_by.
var actorFieldPattern = regexp.MustCompile(`(^|_)(user_?ids?|author\w*|actor\w*|owner\w*|sender\w*|approver\w*|responsible\w*)$|[a-z]ed_?by(_\w+)?$`)

// bodyActorFields - реестр решений по полям тела с именем «кто»: ключ Тип.поле, значение -
// кем это поле является. Действующее лицо в теле не принимается никогда: автор записи,
// голоса, перехода берётся из токена. Сюда попадает только поле, означающее ДРУГОГО
// человека или фильтр чтения; новое поле без записи роняет тест.
var bodyActorFields = map[string]string{
	// Фильтры чтения: сужают выборку, ничего не пишут от чужого имени.
	"AccessDenialFilter.user_id":       "фильтр журнала отказов в доступе, право permission.audit.read",
	"PDAuditFilter.user_id":            "фильтр журнала доступа к ПД, право page.admin.pd_audit",
	"RequestLogsQuery.user_id":         "фильтр журнала запросов, право page.admin.monitoring",
	"PassageHistoryQuery.user_id":      "фильтр журнала проходов по отметившему, гейты журнала поста",
	"ApplicationFilter.sender_user_id": "AND поверх фильтра доступа к заявкам, видимость не расширяет",

	// Другой человек: адресат действия, а не его автор.
	"ForwardUser.user_id":                           "получатель пересылки заявки",
	"RequiredUserInput.user_id":                     "согласующий, назначаемый при подаче заявки",
	"CreateApproverRequest.user_id":                 "назначаемый принимающий, право page.admin.directories",
	"CreateNotificationRequest.user_id":             "адресат уведомления, право page.admin",
	"MergePermissionGroupsRequest.user_id":          "пользователь, чьи группы прав сливаются, право permission.audit.manage",
	"CompleteApplicationRequest.responsible_person": "ФИО контактного лица текстом, не пользователь системы",
}

// TestBodyActorFields_Registered - замок «авторство из токена». Проходит по всем
// Bind/BindAndValidate/Decode хендлеров, раскрывает тип тела со вложенными структурами и
// требует, чтобы каждое поле с именем «кто» было в bodyActorFields с объяснением.
func TestBodyActorFields_Registered(t *testing.T) {
	types := loadStructTypes(t, internalPackageDirs(t)...)
	bound := boundTypes(t, ".", types)
	if len(bound) < 50 {
		t.Fatalf("найдено всего %d привязок тела - разбор хендлеров сломан", len(bound))
	}

	seen := map[string]bool{}
	var missing []string
	for _, b := range bound {
		for _, f := range actorFieldsOf(types, b.typ, map[string]bool{}) {
			seen[f] = true
			if _, ok := bodyActorFields[f]; !ok {
				missing = append(missing, f+" ("+b.where+")")
			}
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("поле тела с именем «кто» без решения: %s", m)
	}
	for k := range bodyActorFields {
		if !seen[k] {
			t.Errorf("запись реестра %s не соответствует ни одному полю тела - убрать", k)
		}
	}
}

type boundType struct {
	typ   string // pkg.Type
	where string // функция хендлера
}

type structDef struct {
	pkg string
	st  *ast.StructType
}

// internalPackageDirs - все пакеты internal/: тело хендлера может быть типом любого из них.
func internalPackageDirs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("..")
	if err != nil {
		t.Fatalf("read internal: %v", err)
	}
	dirs := []string{"."}
	for _, e := range entries {
		if e.IsDir() && e.Name() != "handlers" {
			dirs = append(dirs, filepath.Join("..", e.Name()))
		}
	}
	return dirs
}

func loadStructTypes(t *testing.T, dirs ...string) map[string]structDef {
	t.Helper()
	out := map[string]structDef{}
	for _, dir := range dirs {
		pkg := filepath.Base(dir)
		if dir == "." {
			pkg = "handlers"
		}
		for _, f := range parseDir(t, dir) {
			ast.Inspect(f, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					out[pkg+"."+ts.Name.Name] = structDef{pkg: pkg, st: st}
				}
				return true
			})
		}
	}
	return out
}

func parseDir(t *testing.T, dir string) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	return files
}

// boundTypes находит переменные, в которые хендлеры читают тело или query, и их типы.
// Анонимная структура (var req struct{...}) регистрируется в types под именем
// handlers.<функция>.<переменная>. Тип, которого нет в types, роняет тест: иначе тело из
// незагруженного пакета проходило бы проверку молча.
func boundTypes(t *testing.T, dir string, types map[string]structDef) []boundType {
	t.Helper()
	var out []boundType
	for _, f := range parseDir(t, dir) {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			varTypes := localVarTypes(fn.Body, fn.Name.Name, types)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				target := bindTarget(call)
				if target == "" {
					return true
				}
				typ, ok := varTypes[target]
				if !ok {
					t.Errorf("%s: тип переменной %s в привязке тела не определён - дописать разбор", fn.Name.Name, target)
					return true
				}
				if typ == "" {
					return true
				}
				if _, ok := types[typ]; !ok {
					t.Errorf("%s: тип тела %s не найден среди пакетов internal/ - дописать разбор", fn.Name.Name, typ)
					return true
				}
				out = append(out, boundType{typ: typ, where: fn.Name.Name})
				return true
			})
		}
	}
	return out
}

// bindTarget возвращает имя переменной X в c.Bind(&X), BindAndValidate(c, &X), Decode(&X).
func bindTarget(call *ast.CallExpr) string {
	var arg ast.Expr
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		switch fun.Sel.Name {
		case "Bind", "Decode":
			if len(call.Args) == 1 {
				arg = call.Args[0]
			}
		}
	case *ast.Ident:
		if fun.Name == "BindAndValidate" && len(call.Args) == 2 {
			arg = call.Args[1]
		}
	}
	u, ok := arg.(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return ""
	}
	id, ok := u.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name
}

// localVarTypes: var x T / var x struct{...} / x := T{} / x := &T{}. Значение "" - тип не
// структура (map, срез скаляров), такие тела не проверяются.
func localVarTypes(body *ast.BlockStmt, fn string, types map[string]structDef) map[string]string {
	out := map[string]string{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.ValueSpec:
			for _, name := range s.Names {
				if st, ok := s.Type.(*ast.StructType); ok {
					anon := "handlers." + fn + "." + name.Name
					types[anon] = structDef{pkg: "handlers", st: st}
					out[name.Name] = anon
				} else if s.Type != nil {
					out[name.Name] = typeName(s.Type)
				} else if len(s.Values) > 0 {
					out[name.Name] = compositeType(s.Values[0])
				}
			}
		case *ast.AssignStmt:
			if s.Tok != token.DEFINE {
				return true
			}
			for i, lhs := range s.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || i >= len(s.Rhs) {
					continue
				}
				if typ := compositeType(s.Rhs[i]); typ != "" {
					out[id.Name] = typ
				}
			}
		}
		return true
	})
	return out
}

func compositeType(e ast.Expr) string {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	if cl, ok := e.(*ast.CompositeLit); ok {
		return typeName(cl.Type)
	}
	return ""
}

// typeName: T -> handlers.T, pkg.T -> pkg.T, []T/*T -> как T, встроенные типы, map и
// прочее -> "".
func typeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		if builtinTypes[x.Name] {
			return ""
		}
		return "handlers." + x.Name
	case *ast.SelectorExpr:
		if p, ok := x.X.(*ast.Ident); ok {
			return p.Name + "." + x.Sel.Name
		}
	case *ast.StarExpr:
		return typeName(x.X)
	case *ast.ArrayType:
		return typeName(x.Elt)
	}
	return ""
}

var builtinTypes = map[string]bool{
	"any": true, "bool": true, "byte": true, "rune": true, "string": true, "error": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
}

// actorFieldsOf собирает поля «кто» типа typ, заходя во встроенные, вложенные и
// анонимные структуры. Ключ поля - короткое имя типа и json-имя поля.
func actorFieldsOf(types map[string]structDef, typ string, visiting map[string]bool) []string {
	def, ok := types[typ]
	if !ok || visiting[typ] {
		return nil
	}
	visiting[typ] = true
	defer delete(visiting, typ)
	return actorFieldsOfStruct(types, typ[strings.Index(typ, ".")+1:], def, visiting)
}

func actorFieldsOfStruct(types map[string]structDef, short string, def structDef, visiting map[string]bool) []string {
	var out []string
	for _, field := range def.st.Fields.List {
		name := jsonName(field)
		if name == "-" {
			continue
		}
		if name != "" && actorFieldPattern.MatchString(name) {
			out = append(out, short+"."+name)
		}
		if st := anonStruct(field.Type); st != nil {
			out = append(out, actorFieldsOfStruct(types, short+"."+name, structDef{pkg: def.pkg, st: st}, visiting)...)
			continue
		}
		nested := typeName(field.Type)
		if nested == "" {
			continue
		}
		if strings.HasPrefix(nested, "handlers.") {
			nested = def.pkg + nested[len("handlers"):]
		}
		out = append(out, actorFieldsOf(types, nested, visiting)...)
	}
	return out
}

func anonStruct(e ast.Expr) *ast.StructType {
	switch x := e.(type) {
	case *ast.StructType:
		return x
	case *ast.StarExpr:
		return anonStruct(x.X)
	case *ast.ArrayType:
		return anonStruct(x.Elt)
	}
	return nil
}

// jsonName - имя поля в теле или query: тег json/query/form/param, без тега - имя поля Go
// в нижнем регистре (encoding/json сопоставляет ключи без учёта регистра).
func jsonName(field *ast.Field) string {
	if field.Tag != nil {
		raw, _ := strconv.Unquote(field.Tag.Value)
		tag := reflect.StructTag(raw)
		for _, key := range []string{"json", "query", "form", "param"} {
			if v, ok := tag.Lookup(key); ok {
				return strings.Split(v, ",")[0]
			}
		}
	}
	if len(field.Names) == 1 {
		return strings.ToLower(field.Names[0].Name)
	}
	return ""
}
