import ConfigProvider from 'antd/es/config-provider'
import zhCN from 'antd/es/locale/zh_CN'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import type { ThemeConfig } from 'antd/es/config-provider/context'
import type { ReactNode } from 'react'
import { consoleColors } from '../../styles/themeTokens'

dayjs.locale('zh-cn')

const consoleTheme: ThemeConfig = {
  token: {
    colorPrimary: consoleColors.brand,
    colorInfo: consoleColors.info,
    colorSuccess: consoleColors.success,
    colorWarning: consoleColors.warning,
    colorError: consoleColors.danger,
    colorText: consoleColors.text,
    colorTextSecondary: consoleColors.textSecondary,
    colorBorder: consoleColors.border,
    colorBgBase: consoleColors.canvas,
    colorBgLayout: consoleColors.workspace,
    borderRadius: 2,
    borderRadiusLG: 2,
    controlHeight: 40,
    fontFamily: '"IBM Plex Sans Variable", "Noto Sans SC Variable", "PingFang SC", "Microsoft YaHei", sans-serif',
  },
  components: {
    Button: {
      borderRadius: 2,
      controlHeight: 38,
      primaryShadow: 'none',
    },
    Card: {
      borderRadiusLG: 0,
      boxShadowTertiary: 'none',
    },
    Modal: {
      borderRadiusLG: 2,
    },
    Drawer: {
      borderRadiusLG: 0,
    },
    Table: {
      borderRadius: 0,
      borderRadiusLG: 0,
      cellPaddingBlock: 11,
    },
  },
}

export function AppTheme({ children }: { children: ReactNode }) {
  return <ConfigProvider locale={zhCN} theme={consoleTheme}>{children}</ConfigProvider>
}
