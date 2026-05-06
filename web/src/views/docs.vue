<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../utils/i18n'

type StepItem = {
  title: string
  desc: string
}

type ChoiceItem = {
  title: string
  fit: string
  note: string
}

type EntryItem = {
  title: string
  desc: string
}

type FaqItem = {
  q: string
  a: string
}

const { locale } = useI18n()

const isZh = computed(() => locale.value === 'zh')

const content = computed(() => {
  if (isZh.value) {
    return {
      sections: {
        start: '快速开始',
        mode: '部署模式选择',
        access: '接入方式选择',
        commands: '常用命令',
        entries: '控制台入口',
        faq: '常见问题',
      },
      startLead: '建议按以下顺序完成首次部署和验证。',
      startSteps: [
        {
          title: '1. 启动注册中心',
          desc: '先以单节点方式启动，确认 HTTP 管理接口和 gRPC 数据面都能正常访问，再进入后续接入步骤。',
        },
        {
          title: '2. 选择运行模式',
          desc: '本地开发或验证功能使用 Standalone；优先考虑可用性使用 AP 集群；需要更强一致性控制时使用 CP 集群。',
        },
        {
          title: '3. 接入服务端和客户端',
          desc: '服务提供方先完成注册和心跳，服务消费方再接入发现和订阅，最后通过控制台核对实例状态与发现结果。',
        },
      ] satisfies StepItem[],
      modeLead: '部署模式决定节点协作方式、端口规划以及数据一致性边界。',
      modeChoices: [
        {
          title: 'Standalone',
          fit: '本地开发、接口联调、单节点验证',
          note: '部署最简单，不涉及节点协同。默认管理面使用 `:8500`，默认 gRPC 数据面使用 `:9000`。',
        },
        {
          title: 'Cluster + AP',
          fit: '优先考虑可用性和横向扩展的注册发现集群',
          note: '节点之间复制服务注册数据，适合大多数服务注册与发现场景。',
        },
        {
          title: 'Cluster + CP',
          fit: '需要 Leader 写入和更严格一致性控制的集群',
          note: '需要规划 bootstrap 节点和 Raft 通讯端口，更适合对元数据一致性要求较高的场景。',
        },
      ] satisfies ChoiceItem[],
      accessLead: '接入方式应按项目类型选择，不建议把所有协议视为同一级入口。',
      accessChoices: [
        {
          title: 'pkg/sdk',
          fit: 'Go 服务首选接入方式',
          note: '统一处理注册、发现、订阅和心跳，适合作为新项目的标准接入路径。',
        },
        {
          title: 'HTTP API',
          fit: '脚本调用、平台接入、轻量管理操作',
          note: '适合自动化任务和管理流程，不适合作为高频发现链路的首选接口。',
        },
        {
          title: 'gRPC',
          fit: '强类型客户端、长连接订阅、高频服务发现',
          note: '作为主要数据面协议使用，适合服务进程直接接入。',
        },
        {
          title: 'Nacos / Consul 兼容',
          fit: '存量系统迁移或兼容接入',
          note: '适合作为迁移过渡层，不建议作为新系统的长期主入口。',
        },
      ] satisfies ChoiceItem[],
      commandsLead: '以下命令覆盖最常见的启动和接入基线。',
      entriesLead: '首次部署完成后，通常从以下入口继续管理和排查。',
      entries: [
        {
          title: '服务列表',
          desc: '查看服务名、实例地址、健康状态、订阅关系以及发现结果，是排查注册与发现链路的首要入口。',
        },
        {
          title: '命名空间',
          desc: '隔离不同业务环境或租户边界，避免测试、预发和生产服务互相干扰。',
        },
        {
          title: '节点管理',
          desc: '查看集群成员、节点角色、健康状态和拓扑变化，用于确认节点是否已正确加入集群。',
        },
        {
          title: '访问控制',
          desc: '管理用户、角色和 API Key，统一约束控制台访问权限和开放接口调用权限。',
        },
        {
          title: '系统设置',
          desc: '调整运行模式、存储参数、日志级别以及集群相关配置，用于运维阶段的持续管理。',
        },
      ] satisfies EntryItem[],
      faq: [
        {
          q: '默认端口是什么？',
          a: 'HTTP 管理面默认使用 `:8500`，gRPC 数据面默认使用 `:9000`。CP 集群下还需要单独规划 Raft 端口。',
        },
        {
          q: '什么时候选择 AP，什么时候选择 CP？',
          a: '服务注册与发现链路通常优先使用 AP；如果元数据写入必须由 Leader 严格控制，再选择 CP。',
        },
        {
          q: '新项目应该从哪个接口开始接入？',
          a: 'Go 项目优先从 `pkg/sdk` 接入；脚本或平台侧集成可使用 HTTP API；兼容接口主要用于迁移。',
        },
        {
          q: '控制台里已经看到服务，为什么客户端还发现不到？',
          a: '优先检查客户端连接的是不是正确的 gRPC 地址，其次检查命名空间、服务名、实例健康状态和订阅是否建立成功。',
        },
      ] satisfies FaqItem[],
      serverCommand: `go run ./cmd/server

# or specify config
go run ./cmd/server -config ./configs/standalone.yaml`,
      sdkCommand: `client, err := registry.NewClient(registry.Config{
    Endpoints: []string{"127.0.0.1:9000"},
    Namespace: "default",
})`,
    }
  }

  return {
    sections: {
      start: 'Quick Start',
      mode: 'Choose a Deployment Mode',
      access: 'Choose an Integration Path',
      commands: 'Common Commands',
      entries: 'Console Entry Points',
      faq: 'FAQ',
    },
    startLead: 'A practical order for first-time deployment and verification.',
    startSteps: [
      {
        title: '1. Start the registry',
        desc: 'Begin with a single-node deployment and confirm that both the HTTP management endpoint and the gRPC data plane are reachable.',
      },
      {
        title: '2. Select the runtime mode',
        desc: 'Use Standalone for local development and verification, AP for availability-first clusters, and CP when stronger consistency control is required.',
      },
      {
        title: '3. Integrate providers and consumers',
        desc: 'Register service providers first, then connect consumers for discovery and subscription, and finally verify instance visibility in the console.',
      },
    ] satisfies StepItem[],
    modeLead: 'The deployment mode determines node coordination, port planning, and the consistency model.',
    modeChoices: [
      {
        title: 'Standalone',
        fit: 'Local development, API validation, single-node verification',
        note: 'The simplest deployment path with no node coordination. The default management port is `:8500`, and the default gRPC port is `:9000`.',
      },
      {
        title: 'Cluster + AP',
        fit: 'Availability-first service registration and discovery clusters',
        note: 'Nodes replicate service registration data across the cluster and fit most production discovery scenarios.',
      },
      {
        title: 'Cluster + CP',
        fit: 'Clusters that require leader-based writes and tighter consistency control',
        note: 'Requires explicit bootstrap planning and Raft port coordination. Best suited for stricter metadata consistency requirements.',
      },
    ] satisfies ChoiceItem[],
    accessLead: 'Choose the integration path by project type instead of treating all protocols as equal entry points.',
    accessChoices: [
      {
        title: 'pkg/sdk',
        fit: 'Preferred path for Go services',
        note: 'Handles registration, discovery, subscription, and heartbeat in one client and should be the default path for new Go services.',
      },
      {
        title: 'HTTP API',
        fit: 'Automation scripts, platform integration, lightweight management flows',
        note: 'Well suited for operational workflows and automation, but not the primary choice for high-frequency discovery paths.',
      },
      {
        title: 'gRPC',
        fit: 'Typed clients, long-lived subscriptions, high-frequency discovery',
        note: 'Used as the primary data plane protocol for service processes that connect directly to the registry.',
      },
      {
        title: 'Nacos / Consul Compatibility',
        fit: 'Migration and compatibility scenarios',
        note: 'Useful as a transition layer for existing systems rather than the long-term primary entry point for new projects.',
      },
    ] satisfies ChoiceItem[],
    commandsLead: 'These examples cover the most common startup and integration baseline.',
    entriesLead: 'After initial deployment, these are the main console areas used for day-to-day management and troubleshooting.',
    entries: [
      {
        title: 'Services',
        desc: 'Inspect service names, instance addresses, health status, subscription relationships, and discovery results. This is the primary entry point for troubleshooting registration and discovery.',
      },
      {
        title: 'Namespaces',
        desc: 'Separate environments or tenant boundaries so that development, staging, and production traffic remain isolated.',
      },
      {
        title: 'Nodes',
        desc: 'Review cluster members, node roles, health state, and topology changes to confirm that nodes joined the cluster correctly.',
      },
      {
        title: 'Access Control',
        desc: 'Manage users, roles, and API keys for both console access and external API usage.',
      },
      {
        title: 'Settings',
        desc: 'Adjust runtime mode, storage parameters, logging level, and cluster-related configuration during operations.',
      },
    ] satisfies EntryItem[],
    faq: [
      {
        q: 'What are the default ports?',
        a: 'The HTTP management plane uses `:8500` by default, and the gRPC data plane uses `:9000` by default. CP clusters also require a separate Raft port plan.',
      },
      {
        q: 'When should I choose AP versus CP?',
        a: 'AP is the better default for service registration and discovery. Choose CP only when metadata writes must be controlled through a leader with stricter consistency guarantees.',
      },
      {
        q: 'Which integration path should a new project start with?',
        a: 'Start with `pkg/sdk` for Go services, use the HTTP API for scripts or platform automation, and keep compatibility interfaces mainly for migration work.',
      },
      {
        q: 'Why is the service visible in the console but still not discoverable from the client?',
        a: 'Check the client gRPC endpoint first, then verify the namespace, service name, instance health state, and whether the subscription was established successfully.',
      },
    ] satisfies FaqItem[],
    serverCommand: `go run ./cmd/server

# or specify config
go run ./cmd/server -config ./configs/standalone.yaml`,
    sdkCommand: `client, err := registry.NewClient(registry.Config{
    Endpoints: []string{"127.0.0.1:9000"},
    Namespace: "default",
})`,
  }
})
</script>

<template>
  <article class="docs-article glass-card">
    <section class="article-section first-section">
      <h2>{{ content.sections.start }}</h2>
      <p class="section-lead">{{ content.startLead }}</p>
      <div class="step-list">
        <div v-for="item in content.startSteps" :key="item.title" class="step-item">
          <h3>{{ item.title }}</h3>
          <p>{{ item.desc }}</p>
        </div>
      </div>
    </section>

    <section class="article-section">
      <h2>{{ content.sections.mode }}</h2>
      <p class="section-lead">{{ content.modeLead }}</p>
      <div class="choice-list">
        <div v-for="item in content.modeChoices" :key="item.title" class="choice-item">
          <h3>{{ item.title }}</h3>
          <span class="choice-fit">{{ item.fit }}</span>
          <p>{{ item.note }}</p>
        </div>
      </div>
    </section>

    <section class="article-section">
      <h2>{{ content.sections.access }}</h2>
      <p class="section-lead">{{ content.accessLead }}</p>
      <div class="choice-grid">
        <div v-for="item in content.accessChoices" :key="item.title" class="choice-item">
          <h3>{{ item.title }}</h3>
          <span class="choice-fit">{{ item.fit }}</span>
          <p>{{ item.note }}</p>
        </div>
      </div>
    </section>

    <section class="article-section">
      <h2>{{ content.sections.commands }}</h2>
      <p class="section-lead">{{ content.commandsLead }}</p>
      <div class="code-block">
        <div class="code-title">bash</div>
        <pre><code>{{ content.serverCommand }}</code></pre>
      </div>
      <div class="code-block compact-gap">
        <div class="code-title">go</div>
        <pre><code>{{ content.sdkCommand }}</code></pre>
      </div>
    </section>

    <section class="article-section">
      <h2>{{ content.sections.entries }}</h2>
      <p class="section-lead">{{ content.entriesLead }}</p>
      <div class="entry-grid">
        <div v-for="item in content.entries" :key="item.title" class="entry-item">
          <div class="entry-head">
            <span class="entry-mark"></span>
            <h3>{{ item.title }}</h3>
          </div>
          <p>{{ item.desc }}</p>
        </div>
      </div>
    </section>

    <section class="article-section no-border">
      <h2>{{ content.sections.faq }}</h2>
      <div class="faq-list">
        <div v-for="item in content.faq" :key="item.q" class="faq-item">
          <h3>{{ item.q }}</h3>
          <p>{{ item.a }}</p>
        </div>
      </div>
    </section>
  </article>
</template>

<style scoped>
.glass-card {
  border-radius: 18px;
  border: 1px solid rgba(148, 163, 184, 0.14);
  background: var(--bg-card, rgba(255, 255, 255, 0.94));
  box-shadow: 0 10px 28px rgba(15, 23, 42, 0.05);
}

.docs-article {
  padding: 30px 32px;
}

.article-section h2,
.step-item h3,
.choice-item h3,
.entry-item h3,
.faq-item h3 {
  margin: 0;
  color: var(--text-primary, #0f172a);
}

.section-lead,
.step-item p,
.choice-item p,
.entry-item p,
.faq-item p {
  margin: 0;
  color: var(--text-secondary, #475569);
  line-height: 1.8;
}

.article-section {
  margin-top: 30px;
  padding-top: 26px;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
}

.article-section.first-section {
  margin-top: 0;
  padding-top: 0;
  border-top: 0;
}

.article-section.no-border {
  padding-bottom: 4px;
}

.article-section h2 {
  font-size: 24px;
  line-height: 1.2;
}

.section-lead {
  margin-top: 8px;
  font-size: 14px;
}

.step-list,
.choice-list,
.choice-grid,
.entry-grid,
.faq-list {
  display: grid;
  gap: 14px;
  margin-top: 18px;
}

.choice-grid,
.entry-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.step-item,
.choice-item,
.entry-item,
.faq-item {
  padding: 18px;
  border-radius: 16px;
  border: 1px solid rgba(148, 163, 184, 0.12);
  background: rgba(248, 250, 252, 0.72);
}

.entry-item {
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.9) 0%, rgba(241, 245, 249, 0.82) 100%);
}

.entry-head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.entry-mark {
  width: 8px;
  height: 8px;
  flex: 0 0 8px;
  border-radius: 999px;
  background: var(--accent-blue, #2563eb);
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.12);
}

.step-item h3,
.choice-item h3,
.entry-item h3,
.faq-item h3 {
  font-size: 18px;
  line-height: 1.3;
}

.step-item p,
.choice-item p,
.entry-item p,
.faq-item p {
  margin-top: 8px;
  font-size: 14px;
}

.choice-fit {
  display: inline-flex;
  margin-top: 10px;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(59, 130, 246, 0.08);
  color: var(--accent-blue, #2563eb);
  font-size: 12px;
  font-weight: 600;
}

.code-block {
  overflow: hidden;
  margin-top: 18px;
  border-radius: 16px;
  background: #0f172a;
}

.compact-gap {
  margin-top: 14px;
}

.code-title {
  padding: 10px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  color: #93c5fd;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.code-block pre {
  margin: 0;
  padding: 16px 18px;
  overflow-x: auto;
}

.code-block code {
  color: #e2e8f0;
  font-family: 'Cascadia Code', Consolas, monospace;
  font-size: 13px;
  line-height: 1.75;
  white-space: pre;
}

@media (max-width: 960px) {
  .choice-grid,
  .entry-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .docs-article {
    padding: 20px;
  }

  .article-section h2 {
    font-size: 22px;
  }
}
</style>
