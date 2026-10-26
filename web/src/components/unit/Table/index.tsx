import type { ReactNode, TableHTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from 'react'

import type { EmptyImageName } from '@/assets/images'
import { cn } from '@/helpers/cn'

import { EmptyState } from '@/components/unit/EmptyState'
import { Loading } from '@/components/unit/Loading'


/* ------------------------------ 低阶表格原语 ------------------------------ */

export function TableWrap({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('cf-table-wrap', className)}>{children}</div>
}

export function Table({ children, className, ...rest }: TableHTMLAttributes<HTMLTableElement>) {
  return (
    <table className={cn('cf-table', className)} {...rest}>
      {children}
    </table>
  )
}

export function THead({ children }: { children: ReactNode }) {
  return <thead>{children}</thead>
}

export function TBody({ children, className }: { children: ReactNode; className?: string }) {
  return <tbody className={cn('cf-tbody', className)}>{children}</tbody>
}

export interface TRProps {
  children: ReactNode
  className?: string
  clickable?: boolean
  onClick?: () => void
}

export function TR({ children, className, clickable = false, onClick }: TRProps) {
  return (
    <tr
      className={cn(clickable && 'cf-tr--clickable', className)}
      onClick={onClick}
      tabIndex={clickable ? 0 : undefined}
      onKeyDown={
        clickable
          ? (event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                onClick?.()
              }
            }
          : undefined
      }
    >
      {children}
    </tr>
  )
}

export function TH({ children, className, ...rest }: ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th className={cn('cf-th', className)} {...rest}>
      {children}
    </th>
  )
}

export function TD({ children, className, ...rest }: TdHTMLAttributes<HTMLTableCellElement>) {
  return (
    <td className={cn('cf-td', className)} {...rest}>
      {children}
    </td>
  )
}

/* ------------------------------ 数据表格（复合件） ------------------------------ */

export interface Column<T> {
  /** 列标识，仅用于 React key。 */
  key: string
  title: ReactNode
  /** 单元格渲染；缺省时取 `row[key]`（需自行保证可渲染）。 */
  render?: (row: T) => ReactNode
  width?: string
}

export interface DataTableProps<T> {
  columns: ReadonlyArray<Column<T>>
  rows: ReadonlyArray<T>
  rowKey: (row: T) => string | number
  loading?: boolean
  /** 空态插图（与旧 `EmptyListNotice.imageName` 对齐）。 */
  emptyImage?: EmptyImageName
  emptyTitle?: ReactNode
  emptyNotice?: ReactNode
  onRowClick?: (row: T) => void
  /** 表格下方附加区（通常是 <Pagination />）。 */
  footer?: ReactNode
  className?: string
}

/**
 * 数据表格。
 *
 * AGENTS §4 点名的历史违规"每个页面各写一遍表格 + 分页 + 空态"就是靠这个组件消除的：
 * 页面只需声明列定义与数据，加载态/空态/行交互全部内置。
 */
export function DataTable<T>({
  columns,
  rows,
  rowKey,
  loading = false,
  emptyImage,
  emptyTitle,
  emptyNotice,
  onRowClick,
  footer,
  className,
}: DataTableProps<T>) {
  const showState = loading || rows.length === 0

  return (
    <>
      <TableWrap className={className}>
        <Table>
          <THead>
            <TR>
              {columns.map((column) => (
                <TH key={column.key} style={column.width ? { width: column.width } : undefined}>
                  {column.title}
                </TH>
              ))}
            </TR>
          </THead>
          <TBody>
            {showState ? (
              <tr>
                <td className="cf-table__state" colSpan={columns.length}>
                  {loading ? (
                    <Loading />
                  ) : (
                    <EmptyState imageName={emptyImage} title={emptyTitle} notice={emptyNotice} />
                  )}
                </td>
              </tr>
            ) : (
              rows.map((row) => (
                <TR key={rowKey(row)} clickable={Boolean(onRowClick)} onClick={onRowClick ? () => onRowClick(row) : undefined}>
                  {columns.map((column) => (
                    <TD key={column.key}>
                      {column.render ? column.render(row) : String((row as Record<string, unknown>)[column.key] ?? '')}
                    </TD>
                  ))}
                </TR>
              ))
            )}
          </TBody>
        </Table>
      </TableWrap>
      {footer}
    </>
  )
}

/* ------------------------------ 分页 ------------------------------ */

export interface PaginationProps {
  /** 1 基页码。 */
  page: number
  pageCount: number
  onChange: (page: number) => void
  /** 可选：显示"共 N 条"。 */
  total?: number
  className?: string
}

function pageWindow(page: number, pageCount: number, size = 5): number[] {
  const start = Math.max(1, Math.min(page - Math.floor(size / 2), pageCount - size + 1))
  return Array.from({ length: Math.min(size, pageCount) }, (_, index) => start + index).filter(
    (value) => value >= 1 && value <= pageCount,
  )
}

/** 分页条：hover 文字主色 + 底色 primary-lighter（对应 preset MuiPaginationItem.page）。 */
export function Pagination({ page, pageCount, onChange, total, className }: PaginationProps) {
  if (pageCount <= 1 && total === undefined) {
    return null
  }

  return (
    <div className={cn('cf-pagination', className)}>
      {total !== undefined ? <span className="cf-pagination__info">共 {total} 条</span> : null}
      <button
        type="button"
        className="cf-pagination__btn"
        disabled={page <= 1}
        onClick={() => onChange(page - 1)}
        aria-label="上一页"
      >
        ‹
      </button>
      {pageWindow(page, pageCount).map((value) => (
        <button
          key={value}
          type="button"
          className={cn('cf-pagination__btn', value === page && 'cf-pagination__btn--active')}
          aria-current={value === page ? 'page' : undefined}
          onClick={() => onChange(value)}
        >
          {value}
        </button>
      ))}
      <button
        type="button"
        className="cf-pagination__btn"
        disabled={page >= pageCount}
        onClick={() => onChange(page + 1)}
        aria-label="下一页"
      >
        ›
      </button>
    </div>
  )
}
