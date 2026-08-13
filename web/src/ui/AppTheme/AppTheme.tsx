import ConfigProvider from 'antd/es/config-provider'
import type { ThemeConfig } from 'antd/es/config-provider/context'
import type { ReactNode } from 'react'

const consoleTheme: ThemeConfig = {
  token: {
    colorPrimary: '#FF9D0A',
    colorInfo: '#185F9E',
    colorSuccess: '#07954C',
    colorWarning: '#D97706',
    colorError: '#D92D20',
    colorText: '#1F2329',
    colorTextSecondary: '#68707C',
    colorBorder: '#E3E7ED',
    colorBgBase: '#FFFFFF',
    colorBgLayout: '#F7F8FA',
    borderRadius: 2,
    borderRadiusLG: 2,
    controlHeight: 40,
    fontFamily: '"IBM Plex Sans", "Noto Sans SC", "PingFang SC", "Microsoft YaHei", sans-serif',
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
  return <ConfigProvider theme={consoleTheme}>{children}</ConfigProvider>
}
