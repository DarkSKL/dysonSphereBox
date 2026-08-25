import {defineStore} from 'pinia'
import {account} from '../../wailsjs/go/models'
import {GetCurrentUserInfo, GetSession, Login, Logout} from '../../wailsjs/go/account/AccountService'

const emptyUser = (): account.UserInfo => ({id: 0, uuid: '', userName: '', nickName: '', headerImg: '', authorityId: 0, phone: '', email: '', enable: 0})

export const useAccountStore = defineStore('account', {
  state: () => ({
    initialized: false,
    authenticated: false,
    needChangePassword: false,
    profile: emptyUser(),
  }),
  actions: {
    applySession(session: account.AccountSession): void {
      this.authenticated = session.authenticated
      this.needChangePassword = session.needChangePassword
      this.profile = session.user ?? emptyUser()
    },
    async restoreSession(): Promise<void> {
      this.applySession(await GetSession())
      this.initialized = true
    },
    async login(input: account.LoginRequest): Promise<void> {
      this.applySession(await Login(input))
    },
    async refreshProfile(): Promise<void> {
      this.profile = await GetCurrentUserInfo()
    },
    async logout(): Promise<void> {
      try {
        await Logout()
      } finally {
        this.reset()
      }
    },
    reset(): void {
      this.authenticated = false
      this.needChangePassword = false
      this.profile = emptyUser()
    },
  },
})
