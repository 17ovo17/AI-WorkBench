<template>
  <div class="case-table">
    <div class="filter-bar">
      <el-input :model-value="keyword" placeholder="搜索关键词 / 描述" clearable style="width:280px"
        :prefix-icon="Search" @update:model-value="$emit('update:keyword', $event)"
        @keyup.enter="$emit('reload')" @clear="$emit('reload')" />
      <el-select :model-value="category" placeholder="按分类筛选" clearable style="width:200px"
        @update:model-value="v => { $emit('update:category', v); $emit('reload') }">
        <el-option v-for="c in CATEGORIES" :key="c" :label="c" :value="c" />
      </el-select>
      <el-button @click="$emit('reload')">查询</el-button>
      <span class="result-count">共 {{ total }} 条</span>
    </div>

    <el-table :data="cases" v-loading="loading" stripe style="width:100%" empty-text="暂无案例">
      <el-table-column label="根因分类" width="160">
        <template #default="{ row }">
          <el-tag :type="categoryType(row.root_cause_category)" size="small">{{ row.root_cause_category }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="root_cause_description" label="根因描述" min-width="260" show-overflow-tooltip />
      <el-table-column label="关键词" min-width="200">
        <template #default="{ row }">
          <el-tag v-for="kw in keywordsOf(row)" :key="kw" size="small" effect="plain" style="margin-right:4px">{{ kw }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="160">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" link @click="$emit('detail', row)">查看</el-button>
          <el-button size="small" link @click="$emit('edit', row)">编辑</el-button>
          <el-button size="small" link type="danger" @click="$emit('remove', row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager-bar">
      <el-pagination
        :current-page="page" :page-size="limit" :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        background
        @update:current-page="v => { $emit('update:page', v); $emit('reload') }"
        @update:page-size="v => { $emit('update:limit', v); $emit('reload') }"
      />
    </div>
  </div>
</template>

<script setup>
import { Search } from '@element-plus/icons-vue'
import { CATEGORIES, categoryType } from '../caseHelpers.js'

defineProps({
  cases: { type: Array, default: () => [] },
  total: { type: Number, default: 0 },
  loading: { type: Boolean, default: false },
  page: { type: Number, default: 1 },
  limit: { type: Number, default: 20 },
  keyword: { type: String, default: '' },
  category: { type: String, default: '' },
})

defineEmits(['reload', 'detail', 'edit', 'remove', 'update:page', 'update:limit', 'update:keyword', 'update:category'])

const keywordsOf = (row) => (row.keywords || '').split(',').map(s => s.trim()).filter(Boolean).slice(0, 4)

const formatTime = (iso) => {
  if (!iso) return '-'
  try { return new Date(iso).toLocaleString('zh-CN', { hour12: false }) } catch { return iso }
}
</script>

<style scoped>
.case-table { display: flex; flex-direction: column; gap: 14px; }
.filter-bar { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.result-count { color: var(--muted); font-size: 12px; margin-left: auto; }
.pager-bar { display: flex; justify-content: flex-end; padding: 8px 0 0; }
:deep(.el-table) { background: transparent; }
:deep(.el-table tr), :deep(.el-table th.el-table__cell) { background: transparent !important; }
:deep(.el-table .el-table__cell) { border-bottom: 1px solid rgba(120,140,180,.14); }
</style>
