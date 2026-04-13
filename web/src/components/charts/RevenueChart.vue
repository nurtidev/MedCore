<template>
  <div ref="chartEl" style="width:100%;height:320px;" />
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, ToolboxComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { formatKZT } from '@/utils/format'
import type { RevenueByDay } from '@/types/analytics'

echarts.use([LineChart, GridComponent, TooltipComponent, ToolboxComponent, CanvasRenderer])

const props = defineProps<{ data: RevenueByDay[] }>()
const chartEl = ref<HTMLElement | null>(null)
let chart: echarts.ECharts | null = null

function buildOption(data: RevenueByDay[]) {
  return {
    toolbox: { feature: { saveAsImage: { title: 'PNG' } } },
    tooltip: {
      trigger: 'axis',
      formatter: (params: unknown[]) => {
        const p = (params as { axisValue: string; value: number; data: RevenueByDay }[])[0]
        return `${p.axisValue}<br/>${formatKZT(p.value)}<br/>Платежей: ${p.data?.count ?? ''}`
      },
    },
    grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
    xAxis: { type: 'category', data: data.map((d) => d.date.slice(5)) },
    yAxis: { type: 'value', axisLabel: { formatter: (v: number) => formatKZT(v) } },
    series: [{
      type: 'line', smooth: true, areaStyle: { opacity: 0.15 },
      lineStyle: { color: '#2563eb', width: 2 },
      itemStyle: { color: '#2563eb' },
      data: data.map((d) => ({ value: d.revenue, data: d })),
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
