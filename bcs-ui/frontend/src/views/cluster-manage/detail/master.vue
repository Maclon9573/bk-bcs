<template>
  <div>
    <TkeMaster :cluster-id="clusterId" v-if="provider === 'tencentCloud' || provider === 'tencentPublicCloud'" />
    <GcpMaster :cluster-id="clusterId" v-else-if="provider === 'gcpCloud'" />
    <BluekingMaster :cluster-id="clusterId" v-else-if="provider === 'bluekingCloud'" />
  </div>
</template>
<script lang="ts">
import { computed, defineComponent } from 'vue';

import BluekingMaster from './master-blueking.vue';
import GcpMaster from './master-gcp.vue';
import TkeMaster from './master-tke.vue';

import { useCluster } from '@/composables/use-app';;

export default defineComponent({
  name: 'ClusterMaster',
  components: {
    TkeMaster,
    GcpMaster,
    BluekingMaster,
  },
  props: {
    clusterId: {
      type: String,
      default: '',
      required: true,
    },
  },
  setup(props) {
    const { clusterList } = useCluster();
    const curCluster = computed(() => clusterList.value.find(item => item.clusterID === props.clusterId) || {});
    const provider = computed(() => curCluster.value.provider);

    return {
      provider,
    };
  },
});
</script>
<style lang="postcss" scoped>
@import './form.css';
</style>
