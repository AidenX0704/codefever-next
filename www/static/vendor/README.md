# www/static/vendor

服务端渲染页面（`backend/internal/httpapi/templates/`）与前端产物共用的第三方静态资源。

## 裁剪原则

这里只保留**运行期真正被引用**的文件。其余（源码包、demo、源码映射、SCSS/Less 源文件、
图标 sprite、metadata 等）已在二次开源时移除，使仓库体积从约 23 MB 降到约 3.6 MB、
文件数从 2072 降到 26。

> 裁剪只删除"没有任何引用"的文件，不修改任何保留文件的功能性内容。
> 唯一的例外：`bootstrap.min.css` 与 `bootstrap.bundle.min.js` 末尾的
> `sourceMappingURL` 注释被移除，因为对应的 `.map` 文件已随未压缩源码一并删除，
> 留着会让浏览器 DevTools 持续请求 404。

## 当前保留清单

| 组件 | 版本 | 保留的文件 | 被谁引用 |
| --- | --- | --- | --- |
| Bootstrap | 4.4.1 | `css/bootstrap.min.css`、`js/bootstrap.bundle.min.js` | `header_include.html` / `footer_include.html` |
| Font Awesome Free | 5.13.0 | `css/all.css`、`webfonts/*`（15 个字体文件） | `header_include.html` |
| jQuery | 3.5.0 | `jquery.min.js` | `footer_include.html` |
| jQuery Validation | 1.19.1 | `dist/jquery.validate.min.js` | `footer_include.html` |
| intl-tel-input | — | `css/intlTelInput.css`、`js/intlTelInput.js`、`img/flags.png`、`img/flags@2x.png` | `header_include.html` / `footer_include.html` |
| Toastr | — | `toastr.css`、`toastr.js` | `header_include.html` / `footer_include.html` |

> `webfonts/` 与 `img/flags*.png` 是被保留的 CSS 间接引用的资源，
> 不能单独删除，否则图标与国别旗标会失效。

## 升级第三方组件时

1. 从上游取新版本，只把**运行期需要的文件**覆盖进来；
2. 检查新版本 CSS 里的 `url(...)` 引用，把新引入的资源一并补上；
3. 用 `grep -rn "/static/" backend/internal/httpapi/templates/` 复核引用路径；
4. 删掉本次新增但用不到的 demo / 源码 / 映射文件。

## 许可证

这些组件各自遵循上游协议（Bootstrap MIT、Font Awesome Free 的图标部分为 CC BY 4.0、
字体部分为 SIL OFL 1.1、jQuery MIT、intl-tel-input MIT、Toastr MIT）。
本仓库的整体协议见根目录 [`LICENSE`](../../../LICENSE)。
