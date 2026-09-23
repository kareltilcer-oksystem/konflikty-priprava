import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { restrictToVerticalAxis, restrictToParentElement } from '@dnd-kit/modifiers'
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'

import {
  useDeleteMeeting,
  useMeeting,
  useRemoveItem,
  useReorderItems,
  useSetDone,
  useUpdateItem,
  useUpdateMeeting,
  useDisplayName,
} from '../../api/hooks'
import { ApiError } from '../../api/client'
import type { MeetingDetail, MeetingItem, User } from '../../api/types'
import { cs, itemCount } from '../../i18n/cs'
import { displayLink, formatDate, formatSize, meetingTitle, safeLink } from '../../lib/format'
import { attachmentKind } from '../../lib/format'
import { AttachmentStrip } from '../../components/Attachments'
import { ChevronLeftIcon, CheckIcon, CloseIcon, DragHandleIcon, PlusIcon, SortArrowsIcon } from '../../components/Icons'
import { DoneBadge, EmptyState, LabelChips, Spinner, btn, cx, microLabel } from '../../components/ui'
import { NotFound } from '../../components/NotFoundPage'
import { AddProblemsPicker } from './AddProblemsPicker'
import { DeleteMeetingDialog } from './DeleteMeetingDialog'

/**
 * The agenda page (A7/A8/A9): the product's centrepiece and its only shared
 * artifact.
 *
 * One component, three densities. The admin sees a working screen with drag
 * handles and inline editors; an anonymous visitor sees the same content at
 * roughly twice the type size with no controls at all, because this page is
 * projected on a wall and read from three metres away; and `@media print`
 * turns it into the A4 handout.
 */
export function MeetingPage({ user }: { user: User | null }) {
  const { slug = '' } = useParams()
  const navigate = useNavigate()
  const { data: meeting, isPending, error } = useMeeting(slug)

  const isAdmin = user?.role === 'admin'

  if (isPending) return <Spinner />
  if (error instanceof ApiError && error.isNotFound) {
    return <NotFound detail={cs.errors.notFoundMeeting(slug)} to="/porady" linkLabel={cs.errors.toMeetings} />
  }
  if (!meeting) return <NotFound to="/porady" linkLabel={cs.errors.toMeetings} />

  if (!isAdmin) return <ProjectedAgenda meeting={meeting} />

  // The admin edits on screen but prints the shared agenda.
  //
  // Printing the editing view would lose every note: they live in textareas,
  // which a print stylesheet cannot expand — a two-row box would print clipped,
  // and hiding them prints an agenda with nothing under the headings. FR-R3
  // requires every note expanded, so print renders the projected view instead.
  // The admin is also the person most likely to press Ctrl+P, to carry the
  // agenda into the room.
  return (
    <>
      <div className="print-hide">
        <AdminAgenda meeting={meeting} onDeleted={() => navigate('/porady')} />
      </div>
      <div className="print-only">
        <ProjectedAgenda meeting={meeting} />
      </div>
    </>
  )
}

// ---------------------------------------------------------------- projected

/**
 * The anonymous rendering (A8) — the app's cover image.
 *
 * No controls, no drag handles, no empty editors: an empty note is simply
 * absent. Type is generous because this is what everyone except the admin
 * actually experiences, from across a meeting room.
 */
function ProjectedAgenda({ meeting }: { meeting: MeetingDetail }) {
  return (
    <div className="px-8 py-10 lg:px-[72px]">
      <div className="max-w-[1140px]">
        <PrintHeader meeting={meeting} />

        <div className="flex flex-wrap items-baseline gap-[18px] print:hidden">
          <h1 className="m-0 text-[32px] font-semibold leading-[1.2] tracking-[-0.02em]">
            {meetingTitle(meeting.iso_week, meeting.iso_year)}
          </h1>
          <span className="text-[20px] leading-none text-muted">{formatDate(meeting.meeting_date)}</span>
        </div>

        {meeting.note && (
          <div className="mt-5 border-l-[3px] border-primary bg-primary-note px-5 py-4 print:border-l-0 print:border print:border-[#999] print:bg-transparent print:px-[14px] print:py-[10px]">
            <div className="whitespace-pre-line text-[20px] leading-[1.5] text-prose print:text-[11pt] print:text-black">
              {meeting.note}
            </div>
          </div>
        )}

        {meeting.items.length === 0 ? (
          <EmptyState title={cs.agenda.empty} />
        ) : (
          <div className="mt-[34px] flex flex-col gap-[34px] print:mt-5 print:gap-[18px]">
            {meeting.items.map((item, i) => (
              <ProjectedItem key={item.id} item={item} number={i + 1} first={i === 0} />
            ))}
          </div>
        )}

        <PrintFooter meeting={meeting} />
      </div>
    </div>
  )
}

function ProjectedItem({ item, number, first }: { item: MeetingItem; number: number; first: boolean }) {
  const p = item.problem
  const link = safeLink(p.link)

  return (
    <div
      className={cx(
        'print-item grid grid-cols-[62px_minmax(0,1fr)] items-start gap-5',
        'print:grid-cols-[30px_minmax(0,1fr)] print:gap-[10px]',
        !first && 'print:border-t print:border-[#ccc] print:pt-[14px]',
      )}
    >
      <span className="text-right font-mono text-[34px] font-medium leading-none text-check print:text-left print:font-sans print:text-[13pt] print:font-semibold print:text-black">
        {number}.
      </span>

      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-[10px]">
          <div
            className={cx(
              'text-[27px] leading-[1.3] tracking-[-0.01em] print:text-[13pt] print:text-black',
              p.done ? 'font-normal text-muted line-through' : 'font-semibold',
            )}
          >
            {p.title}
          </div>
          <LabelChips labels={p.labels} size="lg" />
          {p.done && <DoneBadge />}
        </div>

        {p.description && (
          <div className="print-full mt-[10px] max-w-[960px] whitespace-pre-line text-[18px] leading-[1.6] text-secondary print:mt-[5px] print:text-[10.5pt] print:text-black">
            {p.description}
          </div>
        )}

        {link && (
          <>
            <a
              href={link}
              target="_blank"
              rel="noreferrer noopener"
              className="mt-3 inline-block font-mono text-[16px] leading-none text-primary no-underline hover:underline print:hidden"
            >
              {displayLink(link)}
            </a>
            {/* In print a link is readable text, not a blue underline. */}
            <div className="print-only mt-[5px] text-[10pt] text-black">
              {cs.agenda.printLink} {displayLink(link)}
            </div>
          </>
        )}

        {p.attachments.length > 0 && (
          <>
            <div className="mt-[18px] flex flex-wrap gap-[14px] print:hidden">
              {p.attachments.map((a) =>
                attachmentKind(a.content_type) === 'image' ? (
                  <img
                    key={a.id}
                    src={a.url}
                    alt={a.filename}
                    className="h-[170px] w-[280px] rounded-[8px] border border-line bg-surface-tile object-cover"
                  />
                ) : attachmentKind(a.content_type) === 'video' ? (
                  <video
                    key={a.id}
                    src={a.url}
                    controls
                    preload="metadata"
                    className="h-[170px] w-[280px] rounded-[8px] border border-line bg-video object-contain"
                  />
                ) : (
                  <a
                    key={a.id}
                    href={`${a.url}?download=1`}
                    download={a.filename}
                    className="flex h-[170px] w-[280px] flex-col items-center justify-center rounded-[8px] border border-line bg-surface-sunken text-[14px] text-secondary no-underline"
                  >
                    {a.filename}
                    <span className="mt-1 text-[12px] text-muted">{formatSize(a.size_bytes)}</span>
                  </a>
                ),
              )}
            </div>

            {/* Attachments are agenda content, so they print too: an image as
                the image, a video as a labelled still, anything else as a
                line naming the file — never a dead player control. */}
            <div className="print-only mt-[9px]">
              {p.attachments.map((a) => {
                const kind = attachmentKind(a.content_type)
                if (kind === 'image') {
                  return (
                    <img
                      key={a.id}
                      src={a.url}
                      alt={a.filename}
                      className="mb-[6px] max-h-[112px] border border-[#999] object-contain"
                    />
                  )
                }
                return (
                  <div key={a.id} className="mb-[6px] border border-[#999] px-[10px] py-[7px] text-[9.5pt] text-black">
                    {kind === 'video' ? cs.agenda.printVideo : cs.agenda.printAttachment}{' '}
                    {a.filename} · {a.content_type} · {formatSize(a.size_bytes)}
                  </div>
                )
              })}
            </div>
          </>
        )}

        {(item.prep_note || item.action_note) && (
          <div className="mt-5 rounded-[10px] border border-line bg-surface-sunken px-5 py-4 print:mt-[9px] print:border-0 print:bg-transparent print:p-0">
            {item.prep_note && (
              <>
                <div className="text-[13px] font-medium uppercase leading-none tracking-[0.08em] text-muted print:hidden">
                  {cs.agenda.prepNote}
                </div>
                <div className="print-full mt-[9px] whitespace-pre-line text-[21px] leading-[1.5] text-ink print:mt-0 print:text-[10.5pt] print:text-black">
                  <span className="print-only font-semibold">{cs.agenda.prepNote}: </span>
                  {item.prep_note}
                </div>
              </>
            )}

            {item.action_note && (
              <div className="mt-4 border-t border-surface-tile pt-[14px] print:mt-[5px] print:border-0 print:pt-0">
                <div className="text-[13px] font-medium uppercase leading-none tracking-[0.08em] text-faint print:hidden">
                  {cs.agenda.actionNote}
                </div>
                <div className="print-full mt-2 whitespace-pre-line text-[18px] leading-[1.55] text-secondary print:mt-0 print:text-[10pt] print:text-black">
                  <span className="print-only font-semibold">{cs.agenda.actionNote}: </span>
                  {item.action_note}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

/** The print-only page header: title and date above a rule. */
function PrintHeader({ meeting }: { meeting: MeetingDetail }) {
  return (
    <div className="print-only mb-4 flex items-baseline justify-between border-b-2 border-black pb-[10px]">
      <span className="text-[14pt] font-semibold text-black">
        {meetingTitle(meeting.iso_week, meeting.iso_year)}
      </span>
      <span className="text-[10.5pt] text-black">{formatDate(meeting.meeting_date)}</span>
    </div>
  )
}

/** The print-only footer: the app name and the shareable URL. */
function PrintFooter({ meeting }: { meeting: MeetingDetail }) {
  return (
    <div className="print-only mt-6 border-t border-[#ccc] pt-[10px] text-[8pt] text-[#333]">
      {cs.app.name} · /porada/{meeting.slug}
    </div>
  )
}

// -------------------------------------------------------------------- admin

/**
 * The admin rendering (A7).
 *
 * This is a working screen: the admin sits in a 20-minute dev sync and builds
 * the whole agenda here.
 */
function AdminAgenda({ meeting, onDeleted }: { meeting: MeetingDetail; onDeleted: () => void }) {
  const displayName = useDisplayName()
  const reorder = useReorderItems(meeting.slug)
  const updateMeeting = useUpdateMeeting(meeting.slug)
  const deleteMeeting = useDeleteMeeting()

  const [picking, setPicking] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [editingDate, setEditingDate] = useState(false)
  const [date, setDate] = useState(meeting.meeting_date)
  const [dragId, setDragId] = useState<number | null>(null)

  // Local order so the list reacts instantly on drop; the server response
  // replaces it once the write lands.
  const [order, setOrder] = useState(() => meeting.items.map((i) => i.id))
  useEffect(() => {
    setOrder(meeting.items.map((i) => i.id))
  }, [meeting.items])

  const sensors = useSensors(
    // A small distance threshold keeps a click on the handle from starting a
    // drag, so the buttons beside it stay clickable.
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const byId = new Map(meeting.items.map((i) => [i.id, i]))
  const items = order.map((id) => byId.get(id)).filter((i): i is MeetingItem => Boolean(i))
  const dragging = dragId === null ? null : (byId.get(dragId) ?? null)
  const dragFrom = dragId === null ? -1 : meeting.items.findIndex((i) => i.id === dragId)

  function onDragStart(e: DragStartEvent) {
    setDragId(Number(e.active.id))
  }

  function onDragEnd(e: DragEndEvent) {
    setDragId(null)
    const { active, over } = e
    if (!over || active.id === over.id) return
    const from = order.indexOf(Number(active.id))
    const to = order.indexOf(Number(over.id))
    if (from < 0 || to < 0) return

    const next = arrayMove(order, from, to)
    setOrder(next)
    // The server wants every item id exactly once, so the whole order goes —
    // not just the pair that moved.
    reorder.mutate(next, {
      onError: () => setOrder(meeting.items.map((i) => i.id)),
    })
  }

  function saveDate() {
    updateMeeting.mutate({ meeting_date: date }, { onSuccess: () => setEditingDate(false) })
  }

  return (
    <div className="px-8 py-6">
      <div className="max-w-[1040px]">
        <Link
          to="/porady"
          className="print-hide inline-flex items-center gap-[6px] text-[13px] leading-none text-secondary no-underline hover:text-primary"
        >
          <ChevronLeftIcon />
          {cs.agenda.back}
        </Link>

        <div className="mt-3 flex items-start justify-between gap-8">
          <div>
            <h1 className="m-0 text-[23px] font-semibold leading-[1.3] tracking-[-0.01em]">
              {meetingTitle(meeting.iso_week, meeting.iso_year)}
            </h1>
            <div className="mt-2 flex flex-wrap items-center gap-[10px]">
              {editingDate ? (
                <>
                  <input
                    type="date"
                    value={date}
                    onChange={(e) => setDate(e.target.value)}
                    className="h-[28px] rounded-[5px] border border-field px-2 text-[13px] focus-ring"
                  />
                  <button type="button" onClick={saveDate} className={btn.tiny}>
                    {cs.agenda.saveDate}
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setDate(meeting.meeting_date)
                      setEditingDate(false)
                    }}
                    className={btn.tiny}
                  >
                    {cs.form.cancel}
                  </button>
                </>
              ) : (
                <>
                  <span className="text-[13px] leading-none text-secondary">
                    {formatDate(meeting.meeting_date)}
                  </span>
                  {/* Editable because a sync slips; the alternative — delete and
                      recreate — destroys every note and breaks the shared link.
                      Changing it moves the week label, never the URL. */}
                  <button type="button" onClick={() => setEditingDate(true)} className={btn.tiny}>
                    {cs.agenda.editDate}
                  </button>
                </>
              )}
              <span className="block h-[14px] w-px bg-rule-strong" />
              <span className="text-[13px] leading-none text-muted">
                {itemCount(meeting.items.length)} · {cs.meetings.createdBy} {displayName(meeting.created_by)}
              </span>
            </div>
          </div>

          <div className="flex flex-none items-center gap-2">
            <button type="button" onClick={() => setPicking(true)} className={btn.primary}>
              <PlusIcon />
              {cs.agenda.addProblems}
            </button>
            <button type="button" onClick={() => setConfirming(true)} className={btn.dangerOutline}>
              {cs.agenda.deleteMeeting}
            </button>
          </div>
        </div>

        <MeetingNoteEditor meeting={meeting} />

        {dragId !== null && (
          <div className="mt-4 flex items-center gap-2 rounded-[7px] border border-primary-line bg-primary-tint px-[13px] py-[9px]">
            <SortArrowsIcon className="text-primary-ink" />
            <span className="text-[12.5px] leading-none text-primary-ink">{cs.agenda.dragHint}</span>
          </div>
        )}

        {items.length === 0 ? (
          <EmptyState title={cs.agenda.empty} hint={cs.agenda.emptyHint} />
        ) : (
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            modifiers={[restrictToVerticalAxis, restrictToParentElement]}
            onDragStart={onDragStart}
            onDragEnd={onDragEnd}
            onDragCancel={() => setDragId(null)}
          >
            <SortableContext items={order} strategy={verticalListSortingStrategy}>
              <div className="mt-4 flex flex-col">
                {items.map((item, i) => (
                  <SortableAgendaItem
                    key={item.id}
                    item={item}
                    number={i + 1}
                    slug={meeting.slug}
                    // While a drag is in flight every item collapses to one
                    // line, so a twelve-item agenda fits on screen and the
                    // admin can drag item 11 to position 2.
                    collapsed={dragId !== null}
                  />
                ))}
              </div>
            </SortableContext>

            <DragOverlay>
              {dragging && (
                <div className="grid grid-cols-[22px_30px_minmax(0,1fr)] items-center gap-3 rounded-[8px] border border-primary-line bg-white py-[13px] pr-4 shadow-[0_12px_28px_rgb(21_24_28/0.18)] [transform:rotate(-0.25deg)]">
                  <span className="flex justify-center pl-2 text-primary">
                    <DragHandleIcon />
                  </span>
                  <span className="font-mono text-[16px] font-medium leading-[1.3] text-primary-ink">
                    {order.indexOf(dragging.id) + 1}.
                  </span>
                  <span className="flex min-w-0 items-center gap-[10px]">
                    <span className="truncate text-[15px] font-medium leading-[1.4] text-ink">
                      {dragging.problem.title}
                    </span>
                    {dragFrom >= 0 && (
                      <span className="flex-none rounded-[4px] bg-surface-track px-[7px] py-[3px] font-mono text-[11px] font-medium leading-[1.4] text-secondary">
                        {cs.agenda.fromPosition(dragFrom + 1)}
                      </span>
                    )}
                  </span>
                </div>
              )}
            </DragOverlay>
          </DndContext>
        )}
      </div>

      <AddProblemsPicker
        meeting={meeting}
        open={picking}
        onClose={() => setPicking(false)}
      />
      <DeleteMeetingDialog
        meeting={meeting}
        open={confirming}
        onClose={() => setConfirming(false)}
        onConfirm={() => deleteMeeting.mutate(meeting.slug, { onSuccess: onDeleted })}
        pending={deleteMeeting.isPending}
      />
    </div>
  )
}

/** The note about the whole meeting, shown above the agenda. */
function MeetingNoteEditor({ meeting }: { meeting: MeetingDetail }) {
  const update = useUpdateMeeting(meeting.slug)
  const [value, setValue] = useState(meeting.note)
  useEffect(() => setValue(meeting.note), [meeting.note])

  return (
    <div className="mt-5 rounded-[8px] border border-line bg-surface-sunken px-[15px] py-[13px]">
      <label className={microLabel} htmlFor="meeting-note-editor">
        {cs.agenda.meetingNote}
      </label>
      <textarea
        id="meeting-note-editor"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        // Saved on blur rather than on every keystroke: this is a note written
        // in one sitting, and a request per character would be noise.
        onBlur={() => {
          if (value !== meeting.note) update.mutate({ note: value })
        }}
        placeholder={cs.agenda.meetingNotePlaceholder}
        rows={2}
        className="mt-[7px] block w-full resize-y rounded-[6px] border border-transparent bg-white px-[10px] py-2 text-[14px] leading-[1.5] text-prose outline-none transition-colors hover:border-field focus-ring"
      />
    </div>
  )
}

/** One agenda item in the admin view, draggable. */
function SortableAgendaItem({
  item,
  number,
  slug,
  collapsed,
}: {
  item: MeetingItem
  number: number
  slug: string
  collapsed: boolean
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging, isOver } = useSortable({
    id: item.id,
  })
  const setDone = useSetDone()
  const removeItem = useRemoveItem(slug)
  const p = item.problem
  const link = safeLink(p.link)

  const style = { transform: CSS.Transform.toString(transform), transition }

  if (collapsed) {
    return (
      <div ref={setNodeRef} style={style} className={cx('relative', isDragging && 'opacity-40')}>
        {/* Where the item would land. */}
        {isOver && !isDragging && (
          <div className="flex items-center gap-[10px] py-[5px]">
            <span className="block h-[9px] w-[9px] rounded-full bg-primary" />
            <span className="block h-[2px] flex-1 bg-primary" />
            <span className="text-[11px] font-medium uppercase leading-none tracking-[0.06em] text-primary-ink">
              {cs.agenda.dropAt(number)}
            </span>
          </div>
        )}
        <div className="grid grid-cols-[22px_30px_minmax(0,1fr)] items-center gap-3 border-b border-rule py-[13px]">
          <button
            type="button"
            {...attributes}
            {...listeners}
            title={cs.agenda.dragHandle}
            className="flex h-[26px] w-[22px] cursor-grab items-center justify-center text-check focus-ring"
          >
            <DragHandleIcon />
          </button>
          <span className="font-mono text-[16px] font-medium leading-[1.3] text-muted">{number}.</span>
          <span className="truncate text-[15px] font-medium leading-[1.4] text-ink">{p.title}</span>
        </div>
      </div>
    )
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cx(
        'grid grid-cols-[22px_30px_minmax(0,1fr)] items-start gap-3 border-b border-rule py-[18px]',
        isDragging && 'opacity-40',
      )}
    >
      <button
        type="button"
        {...attributes}
        {...listeners}
        title={cs.agenda.dragHandle}
        aria-label={cs.agenda.dragHandle}
        className="flex h-[26px] w-[22px] cursor-grab items-center justify-center text-check focus-ring"
      >
        <DragHandleIcon />
      </button>

      <span className="font-mono text-[17px] font-medium leading-[1.3] text-muted">{number}.</span>

      <div className="min-w-0">
        <div className="flex items-start gap-4">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-[10px]">
              <Link
                to={`/problem/${p.id}`}
                className={cx(
                  'text-[16.5px] leading-[1.4] no-underline hover:underline',
                  p.done ? 'font-normal text-muted line-through' : 'font-semibold text-ink',
                )}
              >
                {p.title}
              </Link>
              <LabelChips labels={p.labels} />
              {p.done && <DoneBadge />}
            </div>

            {p.description && (
              <div className="mt-[6px] whitespace-pre-line text-[13.5px] leading-[1.65] text-secondary">
                {p.description}
              </div>
            )}

            {link && (
              <a
                href={link}
                target="_blank"
                rel="noreferrer noopener"
                className="mt-[7px] inline-block font-mono text-[12.5px] leading-none text-primary no-underline hover:underline"
              >
                {displayLink(link)}
              </a>
            )}
          </div>

          <div className="flex flex-none items-center gap-[6px]">
            {/* The admin ticks problems off from the agenda too, including one
                from weeks ago (PRD 6.2). */}
            <button
              type="button"
              onClick={() => setDone.mutate({ id: p.id, done: !p.done })}
              className={cx(
                'inline-flex h-[28px] items-center gap-[6px] rounded-[6px] px-[9px] text-[12px] leading-none transition-colors focus-ring',
                p.done
                  ? 'border border-done-line bg-done-tint text-done'
                  : 'border border-field bg-white text-secondary hover:border-done-line hover:bg-done-tint hover:text-done',
              )}
            >
              {!p.done && <CheckIcon size={12} />}
              {p.done ? cs.agenda.markOpen : cs.agenda.markDone}
            </button>

            <button
              type="button"
              onClick={() => removeItem.mutate(item.id)}
              title={cs.agenda.removeItem}
              aria-label={cs.agenda.removeItem}
              className="flex h-[28px] w-[28px] items-center justify-center rounded-[6px] border border-field bg-white text-muted transition-colors hover:border-danger-line hover:bg-danger-tint focus-ring"
            >
              <CloseIcon size={13} />
            </button>
          </div>
        </div>

        <AttachmentStrip attachments={p.attachments} />

        <NoteEditors item={item} slug={slug} />
      </div>
    </div>
  )
}

/**
 * The two per-item notes.
 *
 * They are not symmetric and do not look symmetric: the prep note is written
 * before the meeting and read aloud during it, so it gets a proper field; the
 * action note is added days later and is usually empty, so it sits quieter.
 */
function NoteEditors({ item, slug }: { item: MeetingItem; slug: string }) {
  const update = useUpdateItem(slug)
  const [prep, setPrep] = useState(item.prep_note)
  const [action, setAction] = useState(item.action_note)

  useEffect(() => setPrep(item.prep_note), [item.prep_note])
  useEffect(() => setAction(item.action_note), [item.action_note])

  return (
    <>
      <div className="mt-[13px]">
        <label className={microLabel} htmlFor={`prep-${item.id}`}>
          {cs.agenda.prepNote}
        </label>
        <textarea
          id={`prep-${item.id}`}
          value={prep}
          onChange={(e) => setPrep(e.target.value)}
          onBlur={() => {
            if (prep !== item.prep_note) update.mutate({ itemId: item.id, prep_note: prep })
          }}
          placeholder={cs.agenda.prepNotePlaceholder}
          rows={2}
          className="mt-[6px] block w-full resize-y rounded-[7px] border border-line bg-white px-[11px] py-[9px] text-[14.5px] leading-[1.55] text-prose outline-none transition-colors hover:border-[#c9cfd7] focus-ring"
        />
      </div>

      <div className="mt-[9px] flex items-start gap-[9px] border-l-2 border-line-soft pl-[11px]">
        <span className="whitespace-nowrap text-[11px] font-medium uppercase leading-[1.9] tracking-[0.06em] text-faint">
          {cs.agenda.actionNote}
        </span>
        <textarea
          value={action}
          onChange={(e) => setAction(e.target.value)}
          onBlur={() => {
            if (action !== item.action_note) update.mutate({ itemId: item.id, action_note: action })
          }}
          placeholder={cs.agenda.actionNotePlaceholder}
          rows={1}
          aria-label={cs.agenda.actionNote}
          className="flex-1 resize-y rounded-[6px] border border-transparent bg-transparent px-[7px] py-1 text-[13px] leading-[1.6] text-secondary outline-none transition-colors hover:border-line hover:bg-white focus:bg-white focus-ring"
        />
      </div>
    </>
  )
}
