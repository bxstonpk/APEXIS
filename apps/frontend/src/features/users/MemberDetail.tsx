'use client'

import { PauseCircle, PlayCircle } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useState } from 'react'
import { Alert, Button } from '@/components'
import { format, texts } from '@/config'
import { FormSection } from '@/features/forms/FormParts'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { RoleChecklist } from './RoleChecklist'
import type { Member, Role } from './types'

/**
 * จัดการสมาชิก: เปลี่ยนบทบาท (PUT /roles แทนทั้งชุด) และพัก / เปิดใช้งาน (PATCH, ยืนยันก่อน)
 * แก้บัญชีตัวเองไม่ได้ (backend ก็ปฏิเสธ) — แสดงเหตุผลแทนการซ่อน
 */
export function MemberDetail({ apiPath, member, roles, canManage, isSelf }: { apiPath: string; member: Member; roles: Role[]; canManage: boolean; isSelf: boolean }) {
  const t = texts.members
  const router = useRouter()
  const editable = canManage && !isSelf
  const name = member.user.name
  const [roleIds, setRoleIds] = useState(() => new Set(member.role_ids))
  const [busy, setBusy] = useState<'roles' | 'status' | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [rolesError, setRolesError] = useState<string | undefined>()
  const dirty = roleIds.size !== member.role_ids.length || member.role_ids.some((id) => !roleIds.has(id))

  async function saveRoles() {
    if (roleIds.size === 0) return setRolesError(t.fields.rolesRequired)
    setRolesError(undefined)
    setError(null)
    setBusy('roles')
    try {
      await apiRequest(`${apiPath}/${member.user.id}/roles`, { method: 'PUT', body: { role_ids: [...roleIds] } })
      showToast(format(t.rolesSaved, { name }))
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(null)
    }
  }

  async function changeStatus() {
    const suspend = member.status === 'active'
    if (!window.confirm(format(suspend ? t.suspendConfirm : t.activateConfirm, { name }))) return
    setError(null)
    setBusy('status')
    try {
      await apiRequest(`${apiPath}/${member.user.id}`, { method: 'PATCH', body: { status: suspend ? 'suspended' : 'active' } })
      showToast(format(suspend ? t.suspended : t.activated, { name }))
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      {error && <Alert tone="error">{error}</Alert>}
      {!canManage && <Alert>{texts.common.readOnly}</Alert>}
      {canManage && isSelf && <Alert>{t.selfHint}</Alert>}

      <FormSection title={t.sections.roles} description={t.sections.rolesHint}>
        <RoleChecklist roles={roles} selected={roleIds} onChange={setRoleIds} disabled={!editable} error={rolesError} />
        {editable && (
          <div className="flex justify-end gap-2 sm:col-span-2">
            {dirty && (
              <Button variant="ghost" disabled={busy !== null} onClick={() => setRoleIds(new Set(member.role_ids))}>
                {texts.company.discard}
              </Button>
            )}
            <Button loading={busy === 'roles'} loadingText={texts.common.saving} disabled={!dirty || busy !== null} onClick={saveRoles}>
              {texts.common.save}
            </Button>
          </div>
        )}
      </FormSection>

      <FormSection title={t.sections.status} description={t.sections.statusHint}>
        <div className="flex flex-wrap items-center justify-between gap-3 sm:col-span-2">
          <p className="text-sm">
            {t.columns.status}: <span className="font-medium">{t.statuses[member.status]}</span>
          </p>
          {editable && (
            <Button variant={member.status === 'active' ? 'danger' : 'secondary'} loading={busy === 'status'} disabled={busy !== null} onClick={changeStatus}>
              {member.status === 'active' ? <PauseCircle aria-hidden /> : <PlayCircle aria-hidden />}
              {member.status === 'active' ? t.suspend : t.activate}
            </Button>
          )}
        </div>
      </FormSection>
    </div>
  )
}
