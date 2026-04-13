<template>
  <div ref="chartEl" style="width:100%;height:280px;" />
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import * as echarts from 'echarts/core'
import { FunnelChart } from 'echarts/charts'
import { TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([FunnelChart, TooltipComponent, LegendComponent, CanvasRenderer])

const props = defineProps<{
  data: { name: string; value: number }[]
}>()

const chartEl = ref<HTMLElement | null>(null)
let chart: echarts.ECharts | null = null

onMounted(() => {
  if (!chartEl.value) return
  chart = echarts.init(chartEl.value)
  chart.setOption({
    tooltip: { trigger: 'item', formatter: '{b}: {c}' },
    series: [{
      type: 'funnel',
      left: '10%', width: '80%',
      data: props.data,
      label: { position: 'inside', color: '#fff', fontSize: 12 },
      itemStyle: { borderColor: '#fff', borderWidth: 1 },
      emphasis: { label: { fontSize: 14 } },
    }],
  })
})

onUnmounted(() => chart?.dispose())
</script>
