import { useState } from 'react'

import { emptyImages } from '@/assets/images'
import { Avatar } from '@/components/unit/Avatar'
import { Button } from '@/components/unit/Button'
import { Card } from '@/components/unit/Card'
import { Checkbox, Switch } from '@/components/unit/Choices'
import { useConfirm } from '@/components/unit/Confirm'
import { CopyButton } from '@/components/unit/CopyButton'
import { Dialog } from '@/components/unit/Dialog'
import { EmptyState } from '@/components/unit/EmptyState'
import { Form, FormActions, FormGrid, FormItem } from '@/components/unit/Form'
import { IconButton } from '@/components/unit/IconButton'
import { Loading, PageLoading, Spinner } from '@/components/unit/Loading'
import { SelectField } from '@/components/unit/SelectField'
import { DataTable, Pagination, type Column } from '@/components/unit/Table'
import { Tabs } from '@/components/unit/Tabs'
import { Tag } from '@/components/unit/Tag'
import { TextField, TextareaField } from '@/components/unit/TextField'
import { useToast } from '@/components/unit/Toast'
import { Tooltip } from '@/components/unit/Tooltip'
import { color, radius, shadow, space } from '@/theme/tokens'


interface DemoRow {
  name: string
  role: string
  updated: string
}

const DEMO_ROWS: DemoRow[] = [
  { name: 'codefever-community', role: 'Owner', updated: '2026-10-24 17:02' },
  { name: 'gitsvc', role: 'Developer', updated: '2026-10-23 09:11' },
  { name: 'web', role: 'Reporter', updated: '2026-10-22 22:49' },
]

const DEMO_COLUMNS: ReadonlyArray<Column<DemoRow>> = [
  { key: 'name', title: '仓库', render: (row) => <span style={{ color: color.primary }}>{row.name}</span> },
  { key: 'role', title: '角色', render: (row) => <Tag tone="primary">{row.role}</Tag> },
  { key: 'updated', title: '最近更新' },
]

const COLOR_TOKENS: ReadonlyArray<{ label: string; value: string }> = [
  { label: 'primary', value: color.primary },
  { label: 'primary-light', value: color.primaryLight },
  { label: 'primary-dark', value: color.primaryDark },
  { label: 'primary-lighter', value: color.primaryLighter },
  { label: 'secondary', value: color.secondary },
  { label: 'success', value: color.success },
  { label: 'warning', value: color.warning },
  { label: 'danger', value: color.danger },
  { label: 'border', value: color.border },
  { label: 'text', value: color.text },
  { label: 'text-light', value: color.textLight },
  { label: 'text-lighter', value: color.textLighter },
  { label: 'bg-main', value: color.bgMain },
  { label: 'bg-dark', value: color.bgDark },
]

/**
 * 设计系统预览页（`/__design`）。
 *
 * 作用：批 0 的验收载体 —— 所有 unit 组件与设计令牌在同一页呈现，
 * 用于与改造前的 MUI v4 页面并排截图比对（AGENTS §5 的落地检查点）。
 * 生产构建保留也刻意无害：它是纯静态页，不含业务数据。
 */
export default function DesignSystem() {
  const toast = useToast()
  const confirm = useConfirm()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [tab, setTab] = useState<'overview' | 'members'>('overview')
  const [page, setPage] = useState(2)
  const [email, setEmail] = useState('')
  const [checked, setChecked] = useState(true)

  return (
    <div className="page">
      <div>
        <div className="page__title">设计系统</div>
        <div className="page__subtitle">
          自研 tokens + 组件库（B 方案）。所有取值来自旧主题 preset 的现值，见 theme/theme.css 注释。
        </div>
      </div>

      <section className="section">
        <div className="section__title">颜色</div>
        <div className="swatch-grid">
          {COLOR_TOKENS.map((item) => (
            <div className="swatch" key={item.label}>
              <div className="swatch__chip" style={{ background: item.value }} />
              <span>{item.label}</span>
            </div>
          ))}
        </div>
      </section>

      <section className="section">
        <div className="section__title">字号 / 圆角 / 阴影</div>
        <div className="stack">
          <h1>h1 20px bold —— 页面标题</h1>
          <h4>h4 16px bold —— 区块标题</h4>
          <div>body 14px 常规文本（text #2f354d）</div>
          <div style={{ color: color.textLight }}>body-light 14px 次要文本（#7a859e）</div>
          <div style={{ color: color.textLighter }}>caption 14px 最弱文本（#b1b6c3）</div>
          <div className="row">
            <span className="cf-tag" style={{ borderRadius: radius.base }}>
              radius 4
            </span>
            <span className="cf-tag" style={{ borderRadius: radius.sm }}>
              radius 2
            </span>
            <span className="cf-tag" style={{ boxShadow: shadow.bar }}>
              shadow.bar
            </span>
            <span className="cf-tag" style={{ boxShadow: shadow.card }}>
              shadow.card
            </span>
            <span className="cf-tag" style={{ boxShadow: shadow.default }}>
              shadow.default
            </span>
            <span className="cf-tag" style={{ marginLeft: space(2) }}>
              spacing(2) = 16px
            </span>
          </div>
        </div>
      </section>

      <section className="section">
        <div className="section__title">按钮</div>
        <div className="row">
          <Button>主按钮</Button>
          <Button variant="outlined">次按钮</Button>
          <Button variant="text">文字按钮</Button>
          <Button tone="danger">删除</Button>
          <Button tone="danger" variant="outlined">
            删除（描边）
          </Button>
          <Button size="sm">small</Button>
          <Button loading>提交中</Button>
          <Button disabled>禁用</Button>
          <IconButton label="示例图标按钮">⋯</IconButton>
        </div>
      </section>

      <section className="section">
        <div className="section__title">表单控件</div>
        <FormGrid>
          <TextField
            label="邮箱"
            required
            placeholder="name@example.com"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            helperText="登录用邮箱"
          />
          <TextField label="密码" type="password" placeholder="至少 6 位" error="请输入至少 6 位密码" />
          <SelectField
            label="可见性"
            options={[
              { value: 'private', label: '私有' },
              { value: 'public', label: '公开' },
            ]}
            defaultValue="private"
          />
          <TextField label="仓库名" startAdornment="/" placeholder="my-repo" endAdornment=".git" />
        </FormGrid>
        <TextareaField label="描述" placeholder="支持 Markdown" rows={3} />
        <div className="row">
          <Checkbox label="启用 Webhook" checked={checked} onChange={(event) => setChecked(event.target.checked)} />
          <Switch label="邮件通知" defaultChecked />
        </div>
      </section>

      <section className="section">
        <div className="section__title">表单容器</div>
        <Form onSubmit={() => toast.success('表单已提交')}>
          <FormItem label="仓库组名称" required help="5~30 个字符，仅限字母数字与下划线">
            <TextField placeholder="group_name" />
          </FormItem>
          <FormItem label="分支保护规则" error="至少选择一条规则">
            <SelectField options={[{ value: 'none', label: '不保护' }]} />
          </FormItem>
          <FormActions>
            <Button variant="text">取消</Button>
            <Button type="submit">保存</Button>
          </FormActions>
        </Form>
      </section>

      <section className="section">
        <div className="section__title">表格 / 分页 / 空态 / 加载</div>
        <Tabs
          items={[
            { key: 'overview', label: '列表' },
            { key: 'members', label: '空态' },
          ]}
          value={tab}
          onChange={setTab}
        />
        {tab === 'overview' ? (
          <DataTable columns={DEMO_COLUMNS} rows={DEMO_ROWS} rowKey={(row) => row.name} loading={loading} />
        ) : (
          <DataTable
            columns={DEMO_COLUMNS}
            rows={[]}
            rowKey={(row) => row.name}
            emptyImage="repositories-empty.png"
            emptyTitle="还没有仓库"
            emptyNotice="创建第一个仓库，或从其他仓库 fork。"
          />
        )}
        <Pagination page={page} pageCount={5} total={48} onChange={setPage} />
        <div className="row">
          <Button variant="outlined" size="sm" onClick={() => setLoading((value) => !value)}>
            切换加载态
          </Button>
          <Loading />
          <Spinner />
        </div>
      </section>

      <section className="section">
        <div className="section__title">弹窗 / 提示 / 图标与标签</div>
        <div className="row">
          <Button variant="outlined" onClick={() => setDialogOpen(true)}>
            打开对话框
          </Button>
          <Button
            variant="outlined"
            tone="danger"
            onClick={async () => {
              const ok = await confirm({
                title: '删除仓库',
                message: '删除后不可恢复，确认继续？',
                confirmText: '删除',
                tone: 'danger',
              })
              toast.show(ok ? 'success' : 'info', ok ? '已删除（示例）' : '已取消')
            }}
          >
            二次确认
          </Button>
          <Button variant="outlined" onClick={() => toast.error('登录信息失效, 需要重新登陆')}>
            错误提示
          </Button>
          <Button variant="outlined" onClick={() => toast.success('操作成功')}>
            成功提示
          </Button>
          <CopyButton text="0x0101" message="错误码已复制" />
        </div>
        <div className="row">
          <Avatar name="CodeFever" />
          <Avatar name="CF" square />
          <Avatar src={emptyImages['repositories-empty.png']} alt="示例" size="lg" />
          <Tag>默认标签</Tag>
          <Tag tone="primary">main</Tag>
          <Tag tone="success">已合并</Tag>
          <Tag tone="danger">已关闭</Tag>
          <Tag tone="warning">待评审</Tag>
          <Tooltip title="复制提交哈希">悬停查看提示</Tooltip>
        </div>
        <Card hoverable>
          <div style={{ fontWeight: 'bold' }}>卡片（hover 抬升阴影）</div>
          <div style={{ color: color.textLight }}>用于仓库卡片、统计卡片等列表项。</div>
        </Card>
      </section>

      <section className="section">
        <div className="section__title">空态插图素材</div>
        <div className="row">
          {Object.entries(emptyImages).map(([name, src]) => (
            <Tooltip key={name} title={name}>
              <img src={src} alt={name} style={{ width: space(10) }} loading="lazy" />
            </Tooltip>
          ))}
        </div>
      </section>

      <PageLoading text="PageLoading（路由 chunk 到达前的占位）" />
      <EmptyState imageName="commits-empty.png" title="没有提交记录" notice="推送第一个提交后这里会显示提交历史。" />

      <Dialog
        open={dialogOpen}
        title="对话框标题"
        onClose={() => setDialogOpen(false)}
        actions={
          <>
            <Button variant="text" onClick={() => setDialogOpen(false)}>
              取消
            </Button>
            <Button
              onClick={() => {
                setDialogOpen(false)
                toast.success('已保存')
              }}
            >
              确定
            </Button>
          </>
        }
      >
        <div className="stack">
          <TextField label="名称" placeholder="输入名称" />
          <div style={{ color: color.textLight }}>对话框内容区可滚动，超出 90vh 自动出现滚动条。</div>
        </div>
      </Dialog>
    </div>
  )
}
