<template>
  <div ref="chartEl" style="width:100%;height:320px;" />
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import * as echarts from 'echarts/core'
import { BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { DoctorWorkload } from '@/types/analytics'

echarts.use([BarChart, GridComponent, TooltipComponent, CanvasRenderer])

const props = defineProps<{ data: DoctorWorkload[] }>()
const chartEl = ref<HTMLElement | null>(null)
let chart: echarts.ECharts | null = null

function colorFor(pct: number) {
  if (pct >= 80) return '#16a34a'
  if (pct >= 50) return '#ca8a04'
  return '#dc2626'
}

function buildOption(data: DoctorWorkload[]) {
  const sorted = [...data].sort((a, b) => b.workload_percent - a.workload_percent)
  return {
    tooltip: { trigger: 'axis', formatter: '{b}: {c}%' },
    grid: { left: '20%', right: '6%', top: '4%', bottom: '4%' },
    xAxis: { type: 'value', max: 100, axisLabel: { formatter: '{value}%' } },
    yAxis: { type: 'category', data: sorted.map((d) => d.doctor_name) },
    series: [{
      type: 'bar',
      data: sorted.map((d) => ({
        value: Math.round(d.workload_percent),
        itemStyle: { color: colorFor(d.workload_percent) },
      })),
    }],
  }
}

onMounted(() => {
  if (chartEl.value) {
    chart = echarts.init(chartEl.value)
    chart.setOption(buildOption(props.data))
  }
})

watch(() => props.data, (d) => chart?.setOption(buildOption(d)))
onUnmounted(() => chart?.dispose())
</script>
