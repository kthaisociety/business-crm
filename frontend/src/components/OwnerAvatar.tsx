import type { Person } from '../data/mockDeals'

export default function OwnerAvatar({ owner }: { owner: Person }) {
  return (
    <span
      title={owner.name}
      className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-[10px] font-bold ${owner.color}`}
    >
      {owner.initials}
    </span>
  )
}
