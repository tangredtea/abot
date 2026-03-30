'use client'

import { useState, useEffect } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { apiFetch } from '@/lib/api'

import PersonalityTab from './PersonalityTab'

type TabType = 'basic' | 'personality' | 'channels' | 'advanced'

export default function AgentDetailPage() {
  const params = useParams()
  const router = useRouter()
  const agentID = params.id as string
  const [activeTab, setActiveTab] = useState<TabType>('basic')
  const [agent, setAgent] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    fetchAgent()
  }, [agentID])

  const fetchAgent = async () => {
    try {
      const data = await apiFetch(`/api/v1/agents/${agentID}`)
      setAgent(data)
    } catch (error) {
      console.error('Failed to fetch agent:', error)
    } finally {
      setLoading(false)
    }
  }

  const saveAgent = async () => {
    setSaving(true)
    try {
      await apiFetch(`/api/v1/agents/${agentID}`, {
        method: 'PUT',
        body: JSON.stringify(agent),
      })
      alert('保存成功')
    } catch (error) {
      console.error('Failed to save agent:', error)
      alert('保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="text-gray-500">加载中...</div>
      </div>
    )
  }

  if (!agent) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="text-gray-500">Agent 不存在</div>
      </div>
    )
  }

  const tabs = [
    { id: 'basic' as TabType, name: '基本信息', icon: '📋' },
    { id: 'personality' as TabType, name: '人设配置', icon: '🎭' },
    { id: 'channels' as TabType, name: '通道配置', icon: '📡' },
    { id: 'advanced' as TabType, name: '高级设置', icon: '⚙️' },
  ]

  return (
    <div className="flex-1 flex flex-col min-h-0">
      {/* Header */}
      <div className="bg-white border-b shadow-sm">
        <div className="px-4 sm:px-6 lg:px-8 py-4 sm:py-5">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="flex items-center gap-3 sm:gap-4">
              <button
                onClick={() => router.push('/agents')}
                className="text-gray-600 hover:text-gray-900 transition-colors p-1 rounded-lg hover:bg-gray-100"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                </svg>
              </button>
              <div>
                <h1 className="text-xl lg:text-2xl font-bold text-gray-900">Agent 配置</h1>
                <p className="text-sm text-gray-500 mt-1">ID: {agentID}</p>
              </div>
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => router.push('/chat')}
                className="inline-flex items-center gap-2 px-4 py-2 border border-gray-300 text-gray-700 rounded-lg hover:bg-gray-50 transition-colors duration-150 font-medium"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <span className="hidden sm:inline">测试</span>
              </button>
              <button
                onClick={saveAgent}
                disabled={saving}
                className="inline-flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-lg hover:bg-primary-hover transition-colors duration-150 font-medium shadow-sm disabled:opacity-50"
              >
                {saving ? (
                  <>
                    <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                    <span>保存中...</span>
                  </>
                ) : (
                  <>
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                    <span>保存更改</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 flex flex-col sm:flex-row overflow-hidden">
        {/* Sidebar Navigation - Horizontal on mobile, vertical on desktop */}
        <div className="w-full sm:w-64 bg-white border-b sm:border-b-0 sm:border-r overflow-x-auto sm:overflow-x-visible sm:overflow-y-auto">
          <nav className="flex sm:flex-col p-3 sm:p-4 gap-1 sm:space-y-1">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 sm:gap-3 px-3 sm:px-4 py-2.5 sm:py-3 rounded-lg text-left transition-all duration-150 whitespace-nowrap sm:w-full ${
                  activeTab === tab.id
                    ? 'bg-primary-light text-primary font-medium shadow-sm'
                    : 'text-gray-700 hover:bg-gray-50'
                }`}
              >
                <span className="text-lg sm:text-xl">{tab.icon}</span>
                <span className="text-sm sm:text-base">{tab.name}</span>
              </button>
            ))}
          </nav>
        </div>

        {/* Main Content */}
        <div className="flex-1 overflow-y-auto bg-gray-50">
          <div className="px-4 sm:px-6 lg:px-8 py-6">
            <div className="max-w-4xl">
              {activeTab === 'basic' && <BasicTab agent={agent} setAgent={setAgent} />}
              {activeTab === 'personality' && <PersonalityTab agentId={agentID} />}
              {activeTab === 'channels' && <ChannelsTab agent={agent} setAgent={setAgent} />}
              {activeTab === 'advanced' && <AdvancedTab agent={agent} setAgent={setAgent} />}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function BasicTab({ agent, setAgent }: { agent: any; setAgent: (a: any) => void }) {
  return (
    <div className="space-y-5">
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="px-5 sm:px-6 py-4 border-b border-gray-200">
          <h2 className="text-base sm:text-lg font-semibold text-gray-900">基本信息</h2>
        </div>
        
        <div className="p-5 sm:p-6 space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Agent 名称
            </label>
            <input
              type="text"
              value={agent.name || ''}
              onChange={(e) => setAgent({ ...agent, name: e.target.value })}
              className="w-full px-3 py-2 text-base sm:text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-shadow"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              描述
            </label>
            <textarea
              value={agent.description || ''}
              onChange={(e) => setAgent({ ...agent, description: e.target.value })}
              rows={3}
              className="w-full px-3 py-2 text-base sm:text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-shadow resize-none"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              头像
            </label>
            <div className="flex items-center gap-4">
              <div className="text-5xl">{agent.avatar || '🤖'}</div>
              <input
                type="text"
                value={agent.avatar || ''}
                onChange={(e) => setAgent({ ...agent, avatar: e.target.value })}
                placeholder="输入 Emoji"
                className="flex-1 px-4 py-2 text-base sm:text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-shadow"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              状态
            </label>
            <select
              value={agent.status || 'active'}
              onChange={(e) => setAgent({ ...agent, status: e.target.value })}
              className="w-full px-3 py-2 text-base sm:text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-shadow"
            >
              <option value="active">运行中</option>
              <option value="inactive">已停用</option>
            </select>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="px-5 sm:px-6 py-4 border-b border-gray-200">
          <h2 className="text-base sm:text-lg font-semibold text-gray-900">模型配置</h2>
        </div>
        
        <div className="p-5 sm:p-6 space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              LLM 提供商
            </label>
            <select
              value={agent.provider || 'primary'}
              onChange={(e) => setAgent({ ...agent, provider: e.target.value })}
              className="w-full px-3 py-2 text-base sm:text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-shadow"
            >
              <option value="primary">MiniMax (primary)</option>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              模型
            </label>
            <input
              type="text"
              value={agent.model || ''}
              onChange={(e) => setAgent({ ...agent, model: e.target.value })}
              className="w-full px-3 py-2 text-base sm:text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-shadow"
            />
          </div>
        </div>
      </div>
    </div>
  )
}

function ChannelsTab({ agent, setAgent }: { agent: any; setAgent: (a: any) => void }) {
  const config = agent.config || {}
  const channels = config.channels || {}
  
  const updateChannel = (channelId: string, enabled: boolean, channelConfig?: any) => {
    const newChannels = { ...channels }
    if (enabled) {
      newChannels[channelId] = {
        enabled: true,
        config: channelConfig || newChannels[channelId]?.config || {}
      }
    } else {
      delete newChannels[channelId]
    }
    setAgent({
      ...agent,
      config: { ...config, channels: newChannels }
    })
  }

  const updateChannelConfig = (channelId: string, key: string, value: any) => {
    const newChannels = { ...channels }
    if (!newChannels[channelId]) {
      newChannels[channelId] = { enabled: true, config: {} }
    }
    newChannels[channelId].config = {
      ...newChannels[channelId].config,
      [key]: value
    }
    setAgent({
      ...agent,
      config: { ...config, channels: newChannels }
    })
  }

  const allChannels = [
    { 
      id: 'web', 
      name: 'Web 控制台', 
      icon: '🌐', 
      description: '通过 Web 界面与 Agent 交互',
      configFields: []
    },
    { 
      id: 'wecom', 
      name: '企业微信', 
      icon: '💼', 
      description: '接入企业微信机器人',
      configFields: [
        { key: 'corp_id', label: '企业 ID', type: 'text', placeholder: 'ww1234567890abcdef' },
        { key: 'agent_id', label: 'Agent ID', type: 'text', placeholder: '1000002' },
        { key: 'secret', label: 'Secret', type: 'password', placeholder: 'secret_key' },
      ]
    },
    { 
      id: 'telegram', 
      name: 'Telegram', 
      icon: '✈️', 
      description: '接入 Telegram Bot',
      configFields: [
        { key: 'bot_token', label: 'Bot Token', type: 'password', placeholder: '123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11' },
        { key: 'allowed_users', label: '允许的用户 ID（逗号分隔）', type: 'text', placeholder: '123456789,987654321' },
      ]
    },
    { 
      id: 'discord', 
      name: 'Discord', 
      icon: '🎮', 
      description: '接入 Discord Bot',
      configFields: [
        { key: 'bot_token', label: 'Bot Token', type: 'password', placeholder: 'YOUR_DISCORD_BOT_TOKEN_HERE' },
        { key: 'guild_id', label: 'Guild ID（可选）', type: 'text', placeholder: '123456789012345678' },
      ]
    },
    { 
      id: 'feishu', 
      name: '飞书', 
      icon: '🚀', 
      description: '接入飞书机器人',
      configFields: [
        { key: 'app_id', label: 'App ID', type: 'text', placeholder: 'cli_a1234567890abcde' },
        { key: 'app_secret', label: 'App Secret', type: 'password', placeholder: 'secret_key' },
        { key: 'verification_token', label: 'Verification Token', type: 'text', placeholder: 'verification_token' },
      ]
    },
  ]

  return (
    <div className="space-y-5">
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="px-5 sm:px-6 py-4 border-b border-gray-200">
          <h2 className="text-base sm:text-lg font-semibold text-gray-900">通道配置</h2>
          <p className="text-sm text-gray-500 mt-1">选择 Agent 可以接入的通信渠道并配置相关参数</p>
        </div>
        
        <div className="p-5 sm:p-6 space-y-4">
          {allChannels.map((channel) => {
            const isEnabled = !!channels[channel.id]?.enabled
            const channelConfig = channels[channel.id]?.config || {}
            
            return (
              <div key={channel.id} className="border border-gray-200 rounded-lg overflow-hidden">
                {/* Channel Header */}
                <div className="flex items-center justify-between p-4 bg-gray-50 hover:bg-gray-100 transition-colors duration-150">
                  <div className="flex items-center gap-3 flex-1 min-w-0">
                    <span className="text-2xl flex-shrink-0">{channel.icon}</span>
                    <div className="flex-1 min-w-0">
                      <h3 className="font-medium text-gray-900">{channel.name}</h3>
                      <p className="text-sm text-gray-500 truncate">{channel.description}</p>
                    </div>
                  </div>
                  <button
                    onClick={() => updateChannel(channel.id, !isEnabled)}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-150 flex-shrink-0 ml-3 ${
                      isEnabled ? 'bg-primary' : 'bg-gray-200'
                    }`}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-150 ${
                        isEnabled ? 'translate-x-6' : 'translate-x-1'
                      }`}
                    />
                  </button>
                </div>

                {/* Channel Config */}
                {isEnabled && channel.configFields.length > 0 && (
                  <div className="p-4 space-y-3 bg-white border-t border-gray-200">
                    {channel.configFields.map((field) => (
                      <div key={field.key}>
                        <label className="block text-sm font-medium text-gray-700 mb-1">
                          {field.label}
                        </label>
                        <input
                          type={field.type}
                          value={channelConfig[field.key] || ''}
                          onChange={(e) => updateChannelConfig(channel.id, field.key, e.target.value)}
                          placeholder={field.placeholder}
                          className="w-full px-3 py-2 text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-shadow"
                        />
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>

      <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
        <div className="flex gap-3">
          <span className="text-blue-600 text-xl flex-shrink-0">ℹ️</span>
          <div className="text-sm text-blue-800">
            <p className="font-medium mb-1">配置说明</p>
            <ul className="list-disc list-inside space-y-1 text-blue-700">
              <li>Web 控制台默认启用，无需额外配置</li>
              <li>其他通道需要在对应平台创建机器人并获取凭证</li>
              <li>配置保存后，Agent 会立即重新注册并生效</li>
              <li>敏感信息（Token、Secret）会加密存储</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  )
}

function AdvancedTab({ agent, setAgent }: { agent: any; setAgent: (a: any) => void }) {
  const config = agent.config || {}
  
  const updateConfig = (key: string, value: any) => {
    setAgent({
      ...agent,
      config: { ...config, [key]: value },
    })
  }

  return (
    <div className="space-y-5">
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm">
        <div className="px-5 sm:px-6 py-4 border-b border-gray-200">
          <h2 className="text-base sm:text-lg font-semibold text-gray-900">模型参数</h2>
          <p className="text-sm text-gray-500 mt-1">调整模型的生成行为</p>
        </div>
        
        <div className="p-5 sm:p-6 space-y-6">
          <div>
            <div className="flex items-center justify-between mb-3">
              <label className="text-sm font-medium text-gray-700">
                Temperature (温度)
              </label>
              <span className="text-sm font-semibold text-primary">
                {config.temperature || 0.7}
              </span>
            </div>
            <input
              type="range"
              min="0"
              max="2"
              step="0.1"
              value={config.temperature || 0.7}
              onChange={(e) => updateConfig('temperature', parseFloat(e.target.value))}
              className="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-primary"
            />
            <div className="flex justify-between text-xs text-gray-500 mt-2">
              <span>精确 (0)</span>
              <span>平衡 (1)</span>
              <span>创造 (2)</span>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Max Tokens (最大长度)
            </label>
            <input
              type="number"
              value={config.max_tokens || 2048}
              onChange={(e) => updateConfig('max_tokens', parseInt(e.target.value))}
              className="w-full px-3 py-2 text-base sm:text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-shadow"
            />
            <p className="text-xs text-gray-500 mt-2">
              控制 Agent 单次回复的最大长度
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Top P
            </label>
            <input
              type="number"
              min="0"
              max="1"
              step="0.1"
              value={config.top_p || 0.9}
              onChange={(e) => updateConfig('top_p', parseFloat(e.target.value))}
              className="w-full px-3 py-2 text-base sm:text-sm border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-shadow"
            />
            <p className="text-xs text-gray-500 mt-2">
              控制采样的多样性，值越小越保守
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
