'use client';

import { useState, useEffect } from 'react';
import { getToken } from '@/lib/auth';

interface WorkspaceDoc {
  doc_type: string;
  content: string;
  version: number;
}

interface PersonalityTabProps {
  agentId: string;
}

export default function PersonalityTab({ agentId }: PersonalityTabProps) {
  const [docs, setDocs] = useState<Record<string, string>>({
    IDENTITY: '',
    SOUL: '',
    AGENT: '',
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [activeDoc, setActiveDoc] = useState<string>('IDENTITY');

  useEffect(() => {
    loadDocs();
  }, [agentId]);

  const loadDocs = async () => {
    try {
      const token = getToken();
      const response = await fetch('http://localhost:3001/api/v1/workspace/docs', {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) throw new Error('Failed to load workspace docs');

      const data: WorkspaceDoc[] = await response.json();
      const docsMap: Record<string, string> = {};
      data.forEach(doc => {
        docsMap[doc.doc_type] = doc.content;
      });
      setDocs(docsMap);
    } catch (error) {
      console.error('Failed to load workspace docs:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async (docType: string) => {
    setSaving(true);
    try {
      const token = getToken();
      const response = await fetch(`http://localhost:3001/api/v1/workspace/docs/${docType}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          doc_type: docType,
          content: docs[docType],
        }),
      });

      if (!response.ok) throw new Error('Failed to save workspace doc');

      alert(`${docType} 已保存`);
    } catch (error) {
      console.error('Failed to save workspace doc:', error);
      alert('保存失败');
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <div className="p-6">加载中...</div>;
  }

  const docTabs = [
    { key: 'IDENTITY', label: '身份设定', placeholder: '定义 Agent 的身份、角色和基本特征...' },
    { key: 'SOUL', label: '灵魂设定', placeholder: '定义 Agent 的核心价值观、性格特征和行为准则...' },
    { key: 'AGENT', label: 'Agent 配置', placeholder: '定义 Agent 的特殊能力、工作方式和交互规则...' },
  ];

  return (
    <div className="p-6">
      <div className="mb-6">
        <h3 className="text-lg font-medium mb-2">Workspace 文档</h3>
        <p className="text-sm text-gray-500">
          这些文档定义了 Agent 的人格和行为。修改后会立即生效。
        </p>
      </div>

      {/* Tab Navigation */}
      <div className="flex gap-2 mb-6 border-b border-gray-200">
        {docTabs.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveDoc(tab.key)}
            className={`px-4 py-2 font-medium transition-colors ${
              activeDoc === tab.key
                ? 'text-primary-600 border-b-2 border-primary-600'
                : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Active Doc Editor */}
      {docTabs.map(tab => (
        <div key={tab.key} className={activeDoc === tab.key ? 'block' : 'hidden'}>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                {tab.label}
              </label>
              <textarea
                value={docs[tab.key] || ''}
                onChange={(e) => setDocs({ ...docs, [tab.key]: e.target.value })}
                placeholder={tab.placeholder}
                rows={20}
                className="w-full px-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent font-mono text-sm"
              />
            </div>

            <div className="flex justify-end">
              <button
                onClick={() => handleSave(tab.key)}
                disabled={saving}
                className="px-6 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
              >
                {saving ? '保存中...' : '保存'}
              </button>
            </div>
          </div>
        </div>
      ))}

      {/* Documentation */}
      <div className="mt-8 p-4 bg-blue-50 rounded-lg">
        <h4 className="font-medium text-blue-900 mb-2">📖 文档说明</h4>
        <ul className="text-sm text-blue-800 space-y-1">
          <li><strong>IDENTITY</strong>: Agent 的身份、角色定义</li>
          <li><strong>SOUL</strong>: Agent 的核心价值观、性格和行为准则</li>
          <li><strong>AGENT</strong>: Agent 的特殊能力和工作规则</li>
        </ul>
      </div>
    </div>
  );
}
