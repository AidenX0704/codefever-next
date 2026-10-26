package httpapi

// DocMenuItem 是文档菜单项（由 web_doc.go 解析 index.md 得到，供 SSR 与 JSON 端点共用）。
//
// 2026-10 前端重写后，SSR 模板已删除，但文档目录解析仍然复用这个结构
// （JSON 端点 /api/doc/tree 直接由它生成菜单树）。
type DocMenuItem struct {
	// Name 是菜单显示名。
	Name string
	// Path 是菜单对应的目录路径。
	Path string
	// Submenu 是二级菜单；为空表示该项没有子菜单。
	Submenu []DocMenuItem
}
