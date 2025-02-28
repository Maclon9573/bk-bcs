<!-- eslint-disable vue/multi-word-component-names -->
<!-- eslint-disable max-len -->
<template>
  <div class="biz-content">
    <Header hide-back title="LoadBalancers" :desc="$t('deploy.templateset.officialIngressNginx')" />
    <div class="biz-content-wrapper" style="padding: 0;" v-bkloading="{ isLoading: isInitLoading }">
      <div class="biz-panel-header">
        <div class="left">
          <bk-button type="primary" icon="plus" @click.stop.prevent="createLoadBlance">
            {{$t('deploy.templateset.newLoadBalancer')}}
          </bk-button>
        </div>
        <div class="right">
          <ClusterSelectComb
            :placeholder="$t('deploy.templateset.searchClusterName')"
            :search.sync="searchKeyword"
            :cluster-id.sync="searchScope"
            @search-change="searchLoadBalance"
            @refresh="refresh" />
        </div>
      </div>
      <div class="biz-loadbalance">
        <div class="biz-table-wrapper">
          <bk-table
            :size="'medium'"
            :data="curPageData"
            :pagination="pageConf"
            v-bkloading="{ isLoading: isPageLoading && !isInitLoading }"
            @page-limit-change="handlePageLimitChange"
            @page-change="handlePageChange">
            <bk-table-column :label="$t('generic.label.cluster1')" min-width="150">
              <template slot-scope="props">
                <bcs-popover :content="props.row.cluster_id" placement="top">
                  <div class="cluster-name">{{props.row.cluster_name}}</div>
                </bcs-popover>
              </template>
            </bk-table-column>
            <bk-table-column :label="$t('k8s.namespace')" min-width="150">
              <template slot-scope="props">
                {{props.row.namespace_name || '--'}}
              </template>
            </bk-table-column>
            <bk-table-column :label="$t('deploy.templateset.chartNameAndVersion')" min-width="150">
              <template slot-scope="props">
                <div class="chart-info" v-if="props.row.chart">
                  <p>{{$t('generic.label.name')}}：{{props.row.chart.name || '--'}}</p>
                  <p>{{$t('generic.label.version')}}：{{props.row.chart.version || '--'}}</p>
                </div>
                <template v-else>--</template>
              </template>
            </bk-table-column>
            <bk-table-column :label="$t('cluster.labels.updatedAt')" min-width="150">
              <template slot-scope="props">
                {{formatDate(props.row.updated)}}
              </template>
            </bk-table-column>
            <bk-table-column :label="$t('generic.label.updator')" min-width="150">
              <template slot-scope="props">
                {{props.row.updator}}
              </template>
            </bk-table-column>
            <bk-table-column :label="$t('generic.label.action')" min-width="150">
              <template slot-scope="props">
                <a href="javascript:void(0);" class="bk-text-button" @click.stop.prevent="editLoadBalance(props.row)">{{$t('generic.button.update')}}</a>
                <a href="javascript:void(0);" class="bk-text-button" @click.stop.prevent="removeLoadBalance(props.row)">{{$t('generic.button.delete')}}</a>
              </template>
            </bk-table-column>
            <template #empty>
              <BcsEmptyTableStatus :type="searchKeyword ? 'search-empty' : 'empty'" @clear="searchKeyword = ''" />
            </template>
          </bk-table>
        </div>
      </div>
    </div>

    <bk-sideslider
      :quick-close="false"
      :is-show.sync="loadBalanceSlider.isShow"
      :title="loadBalanceSlider.title"
      :width="700"
      @hidden="hideLoadBalanceSlider">
      <div class="p30" slot="content">
        <div class="bk-form bk-form-vertical mb20">
          <div class="bk-form-item is-required">
            <div class="bk-form-content">
              <label class="bk-label">{{$t('generic.label.cluster1')}}：</label>
              <div class="bk-form-content">
                <bk-selector
                  style="width: 100%;"
                  :field-type="'cluster'"
                  :placeholder="$t('generic.placeholder.select')"
                  :setting-key="'cluster_id'"
                  :display-key="'longName'"
                  :is-link="true"
                  :disabled="!!curLoadBalance.id || !!curClusterId"
                  :selected.sync="curLoadBalance.cluster_id"
                  :list="clusterList">
                </bk-selector>
              </div>
            </div>
          </div>

          <div class="bk-form-item is-required mt15">
            <div class="head">
              <label class="bk-label">{{$t('deploy.templateset.nodeIP')}}：</label>
              <bk-button type="primary" size="small" @click="showNodeSelector">{{$t('cluster.nodeList.create.text')}}</bk-button>
            </div>
            <table class="bk-table biz-data-table has-table-bordered" style="border-bottom: none;">
              <thead>
                <tr>
                  <th>IP</th>
                  <th style="width: 160px;">{{$t('generic.label.action')}}</th>
                </tr>
              </thead>
              <tbody>
                <template v-if="curLoadBalance.node_list.length">
                  <tr v-for="(node, index) in curLoadBalance.node_list" :key="index">
                    <td>
                      {{node.inner_ip}}
                    </td>
                    <td>
                      <a href="javascript:void(0);" class="bk-text-button" @click="removeNode(index)">{{$t('generic.button.delete')}}</a>
                    </td>
                  </tr>
                </template>
                <template v-else>
                  <tr>
                    <td colspan="2">
                      <bcs-exception type="empty" scene="part"></bcs-exception>
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>
          </div>

          <div class="bk-form-item">
            <div class="bk-form-content">
              <label class="bk-label">
                {{$t('deploy.templateset.selectVersion')}}：
                <bcs-popover :content="$t('deploy.templateset.selectChartVersion')" placement="right">
                  <i class="bcs-icon bcs-icon-question-circle"></i>
                </bcs-popover>
              </label>
              <div class="bk-form-content">
                <bk-selector
                  style="width: 100%; z-index: 1113;"
                  :placeholder="$t('generic.placeholder.select')"
                  :setting-key="'id'"
                  :display-key="'version'"
                  :is-link="true"
                  :selected.sync="curLoadBalanceChartId"
                  :list="chartVersionList">
                </bk-selector>
              </div>
            </div>
          </div>

          <div class="bk-form-item mt15">
            <label class="bk-label">{{$t('deploy.templateset.valuesContent')}}：</label>
            <div class="bk-form-content">
              <CodeEditor
                lang="yaml"
                :width="'100%'"
                :height="460"
                v-model="curLoadBalance.values"
                full-screen
                @init="editorInitAfter">
              </CodeEditor>
            </div>
          </div>

          <div class="bk-form-item mt25">
            <bk-button type="primary" :loading="isDataSaveing" @click="saveLoadBalance">{{$t('generic.button.save')}}</bk-button>
            <bk-button :disabled="isDataSaveing" @click="hideLoadBalanceSlider">{{$t('generic.button.cancel')}}</bk-button>
          </div>
        </div>
      </div>
    </bk-sideslider>

    <node-selector
      ref="bkNodeSelector"
      :selected="curLoadBalance.node_list"
      @selected="handlerSelectNode">
    </node-selector>
  </div>
</template>

<script>
import yamljs from 'js-yaml';

import { catchErrorHandler, formatDate } from '@/common/util';
import ClusterSelectComb from '@/components/cluster-selector/cluster-select-comb.vue';
import Header from '@/components/layout/Header.vue';
import CodeEditor from '@/components/monaco-editor/new-editor.vue';
import nodeSelector from '@/components/node-selector';

export default {
  components: {
    CodeEditor,
    nodeSelector,
    Header,
    ClusterSelectComb,
  },
  data() {
    return {
      formatDate,
      isPageLoading: false,
      pageConf: {
        total: 0,
        totalPage: 1,
        pageSize: 5,
        curPage: 1,
        show: true,
        limit: 10,
      },
      curLoadBalance: {
        id: '',
        name: '',
        namespace: '',
        project_id: '',
        cluster_id: '',
        protocol: {
          http: {
            port: 80,
            isUse: true,
          },
          https: {
            port: 443,
            isUse: true,
          },
        },
        node_list: [],
        values: '',
      },
      curLoadBalanceChartId: '',
      statusTimer: [],
      nameSpaceClusterList: [],
      isAllDataLoad: false,
      searchKeyword: '',
      searchScope: '',
      isInitLoading: true,
      isDataSaveing: false,
      isLoadBalanceLoading: false,
      prmissions: {},
      clusterIndex: 0,
      loadBalanceSlider: {
        title: '',
        isShow: false,
      },
      chartVersionList: [],
      aceEditor: null,
      curPageData: [],
    };
  },
  computed: {
    isEn() {
      return this.$store.state.isEn;
    },
    varList() {
      return this.$store.state.variable.varList;
    },
    projectId() {
      return this.$route.params.projectId;
    },
    loadBalanceList() {
      let list = Object.assign([], this.$store.state.network.loadBalanceList);
      list = this.formatDataToClient(list);
      return list;
    },
    clusterList() {
      const { clusterList } = this.$store.state.cluster;
      const list = clusterList.map((cluster) => {
        cluster.longName = `${cluster.name}(${cluster.cluster_id})`;
        return cluster;
      });
      return list.filter(item => item.clusterType !== 'virtual' && !item.is_shared);
    },
    searchScopeList() {
      const { clusterList } = this.$store.state.cluster;
      const results = clusterList.map(item => ({
        id: item.cluster_id,
        name: item.name,
      }));

      return results;
    },
    curProject() {
      return this.$store.state.curProject;
    },
    isClusterDataReady() {
      return this.$store.state.cluster.isClusterDataReady;
    },
    curClusterId() {
      return this.$store.getters.curClusterId;
    },
  },
  watch: {
    loadBalanceList() {
      const data = this.getDataByPage(this.pageConf.current);
      this.curPageData = this.formatDataToClient(data);
    },
    curClusterId() {
      this.searchScope = this.curClusterId;
      this.getLoadBalanceList();
    },

    async curLoadBalanceChartId(chartId) {
      await this.handlerSelectChart(chartId);
    },
  },
  mounted() {
    this.initPageConf();
    this.getLoadBalanceList();
  },
  methods: {
    /**
             * 刷新列表
             */
    refresh() {
      this.pageConf.current = 1;
      this.isPageLoading = true;
      this.getLoadBalanceList();
    },

    /**
             * 分页大小更改
             *
             * @param {number} pageSize pageSize
             */
    handlePageLimitChange(pageSize) {
      this.pageConf.current = pageSize;
      this.pageConf.current = 1;
      this.initPageConf();
      this.handlePageChange();
    },

    /**
             * 切换页面时回调
             */
    leaveCallback() {
      for (const key of Object.keys(this.statusTimer)) {
        clearInterval(this.statusTimer[key]);
      }
      this.$store.commit('network/updateLoadBalanceList', []);
    },

    /**
             * 显示节点选择器
             */
    showNodeSelector() {
      if (!this.curLoadBalance.cluster_id) {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('deploy.templateset.selectCluster'),
        });
        return false;
      }
      this.$refs.bkNodeSelector.openDialog(this.curLoadBalance.cluster_id);
    },

    /**
             * 选择节点
             * @param  {object} data 节点
             */
    handlerSelectNode(data) {
      const nodeList = data.map(item => ({
        id: item.inner_ip,
        inner_ip: item.inner_ip,
        unshared: false,
      }));

      this.curLoadBalance.node_list = nodeList;
    },

    /**
             * 删除节点
             * @param  {number} index 节点索引
             */
    removeNode(index) {
      this.curLoadBalance.node_list.splice(index, 1);
    },

    /**
             * 创建新的LB
             */
    async createLoadBlance() {
      this.nameSpaceSelectedList = [];
      this.curLoadBalance = {
        id: '',
        name: '',
        namespace: '',
        project_id: this.projectId,
        cluster_id: this.curClusterId || '',
        protocol: {
          http: {
            port: 80,
            isUse: true,
          },
          https: {
            port: 443,
            isUse: true,
          },
        },
        node_list: [],
        values: '',
      };
      this.loadBalanceSlider.title = this.$t('deploy.templateset.newLoadBalancer');
      this.loadBalanceSlider.isShow = true;

      try {
        const res = await this.$store.dispatch('network/getChartVersions', {
          projectId: this.projectId,
        });
        this.chartVersionList = res.data || [];
        // eslint-disable-next-line @typescript-eslint/prefer-optional-chain
        this.curLoadBalanceChartId = (this.chartVersionList[0] || {}).id || '';
      } catch (e) {
        catchErrorHandler(e, this);
      }
    },

    /**
             * 编辑LB
             * @param  {object} loadBalance loadBalance
             * @param  {number} index 索引
             */
    async editLoadBalance(loadBalance) {
      this.loadBalanceSlider.title = this.$t('deploy.templateset.editLoadBalancer');
      this.loadBalanceSlider.isShow = true;
      const { projectId } = this;
      const projectKind = this.curProject.kind;
      const loadBalanceId = loadBalance.id;

      this.nameSpaceSelectedList = [];
      this.isDataSaveing = true;

      try {
        const res = await this.$store.dispatch('network/getLoadBalanceDetail', {
          projectId,
          loadBalanceId,
          projectKind,
        });

        const curLoadBalance = res.data;
        if (!curLoadBalance) {
          return;
        }
        curLoadBalance.node_list = JSON.parse(curLoadBalance.ip_info);
        this.curLoadBalance = Object.assign({}, curLoadBalance);
        await this.handlerSelectCluster(this.curLoadBalance.cluster_id);
      } catch (e) {
        catchErrorHandler(e, this);
      } finally {
        this.isDataSaveing = false;
      }
    },

    /**
             * 选择集群回调
             * @param  {number}  index 集群索引（ID）
             */
    async handlerSelectCluster(clusterId) {
      const { projectId } = this;
      if (projectId && clusterId) {
        try {
          const params = {
            cluster_id: clusterId,
            namespace: this.curLoadBalance.namespace || '',
          };
          const res = await this.$store.dispatch('network/getChartVersions', {
            projectId,
            params,
          });
          this.chartVersionList = res.data || [];

          this.curLoadBalanceChartId = this.chartVersionList[0]?.id || '';
        } catch (e) {
          catchErrorHandler(e, this);
        }
      } else {
        this.chartVersionList = [];
      }
    },

    /**
             * 选择chart版本回调
             * @param {number} chartId chart_id
             */
    async handlerSelectChart(chartId) {
      const data = this.chartVersionList.find(item => item.id === chartId);
      const { projectId } = this;
      if (projectId && chartId && data) {
        try {
          this.curLoadBalance.values = '';
          const params = {
            version: data.version,
            namespace: this.curLoadBalance.namespace || 'default',
            cluster_id: chartId === -1 ? this.curLoadBalance.cluster_id : undefined,
          };
          const res = await this.$store.dispatch('network/getChartDetails', {
            projectId,
            params,
          });
          const files = res.data.files || {};
          Object.keys(files).forEach((filesKey) => {
            const keys = filesKey.split('/');
            if (keys[keys.length - 1] === 'values.yaml') {
              this.curLoadBalance.values = files[filesKey];
            }
          });
          this.aceEditor.setValue(this.curLoadBalance.values);
        } catch (e) {
          catchErrorHandler(e, this);
        }
      } else {
        this.curLoadBalance.values = '';
      }
    },

    /**
             * 删除LB
             * @param  {object} loadBalance loadBalance
             * @param  {number} index 索引
             */
    async removeLoadBalance(loadBalance) {
      // eslint-disable-next-line @typescript-eslint/no-this-alias
      const self = this;
      const { projectId } = this;
      const projectKind = this.curProject.kind;
      const loadBalanceId = loadBalance.id;
      this.$bkInfo({
        title: this.$t('generic.title.confirmDelete'),
        clsName: 'biz-remove-dialog',
        content: this.$createElement('p', {
          class: 'biz-confirm-desc',
        }, this.$t('deploy.templateset.confirmDeleteLoadBalancer')),
        async confirmFn() {
          self.isPageLoading = true;

          try {
            await self.$store.dispatch('network/removeLoadBalance', {
              projectId,
              loadBalanceId,
              projectKind,
            });
            self.$bkMessage({
              theme: 'success',
              message: self.$t('generic.msg.success.delete'),
            });
            self.getLoadBalanceList();
          } catch (e) {
            catchErrorHandler(e, this);
            self.isPageLoading = false;
          }
        },
      });
    },

    /**
             * 清空搜索
             */
    clearSearch() {
      this.searchKeyword = '';
      this.searchLoadBalance();
    },

    /**
             * 搜索LB
             */
    searchLoadBalance() {
      const keyword = this.searchKeyword?.trim().toLowerCase();
      const keyList = ['cluster_name', 'name'];
      let list = this.$store.state.network.loadBalanceList;
      let results = [];

      if (this.searchScope) {
        list = list.filter(item => item.cluster_id === this.searchScope);
      }

      results = list.filter((item) => {
        for (const key of keyList) {
          if ((item[key]?.toLowerCase()).indexOf(keyword) > -1) {
            return true;
          }
        }
        return false;
      });
      this.loadBalanceList.splice(0, this.loadBalanceList.length, ...results);
      this.pageConf.current = 1;
      this.initPageConf();
      this.curPageData = this.getDataByPage(this.pageConf.current);
    },

    /**
             * 初始化分页配置
             */
    initPageConf() {
      const total = this.loadBalanceList.length;
      this.pageConf.count = total;
      this.pageConf.current = 1;
      this.pageConf.totalPage = Math.ceil(total / this.pageConf.current);
    },

    /**
             * 重新加载当前页
             */
    reloadCurPage() {
      this.initPageConf();
      this.curPageData = this.getDataByPage(this.pageConf.current);
    },

    /**
             * 获取页数据
             * @param  {number} page 页
             * @return {object} data lb
             */
    getDataByPage(page) {
      // 如果没有page，重置
      if (!page) {
        // eslint-disable-next-line no-multi-assign
        this.pageConf.current = page = 1;
      }
      let startIndex = (page - 1) * this.pageConf.current;
      let endIndex = page * this.pageConf.current;
      this.isPageLoading = true;
      if (startIndex < 0) {
        startIndex = 0;
      }
      if (endIndex > this.loadBalanceList.length) {
        endIndex = this.loadBalanceList.length;
      }
      setTimeout(() => {
        this.isPageLoading = false;
      }, 200);
      return this.loadBalanceList.slice(startIndex, endIndex);
    },

    /**
             * 分页改变回调
             * @param  {number} page 页
             */
    handlePageChange(page = 1) {
      this.isPageLoading = true;
      this.pageConf.current = page;
      const data = this.getDataByPage(page);
      this.curPageData = JSON.parse(JSON.stringify(data));
    },

    /**
             * 隐藏lb侧面板
             * @return {[type]} [description]
             */
    hideLoadBalanceSlider() {
      this.curLoadBalance = {
        id: '',
        name: '',
        namespace: '',
        project_id: this.projectId,
        cluster_id: this.curClusterId || '',
        protocol: {
          http: {
            port: 80,
            isUse: true,
          },
          https: {
            port: 443,
            isUse: true,
          },
        },
        node_list: [],
        values: '',
      };

      this.loadBalanceSlider.isShow = false;

      this.curLoadBalanceChartId = '';
      this.aceEditor.setValue('');
    },

    /**
             * 获取loadBalanceList
             */
    async getLoadBalanceList() {
      try {
        const project = this.curProject;
        const params = {
          cluster_id: this.searchScope,
        };
        this.isPageLoading = true;
        await this.$store.dispatch('network/getLoadBalanceListByPage', {
          project,
          params,
        });
        this.isAllDataLoad = true;
        this.initPageConf();
        // 如果有搜索关键字，继续显示过滤后的结果
        if (this.searchKeyword) {
          this.searchLoadBalance();
        }
      } catch (e) {
        catchErrorHandler(e, this);
      } finally {
        // 晚消失是为了防止整个页面loading和表格数据loading效果叠加产生闪动
        setTimeout(() => {
          this.isPageLoading = false;
          this.isInitLoading = false;
        }, 200);
      }
    },

    /**
             * 检查提交的数据
             * @return {boolean} true/false 是否合法
             */
    checkData1() {
      const data = this.formatDataToServer();
      if (!data.cluster_id) {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('deploy.templateset.selectCluster'),
          delay: 5000,
        });
        return false;
      }

      if (!data.namespace_id) {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('dashboard.ns.validate.emptyNs'),
          delay: 5000,
        });
        return false;
      }

      if (data.protocols.http.isUse && !data.protocols.http.port) {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('deploy.templateset.enterHttpPort'),
          delay: 5000,
        });
        return false;
      }

      if (data.protocols.https.isUse && !data.protocols.https.port) {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('deploy.templateset.enterHttpsPort'),
          delay: 5000,
        });
        return false;
      }

      if (data.ip_info === '{}') {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('deploy.templateset.addNode'),
          delay: 5000,
        });
        return false;
      }

      return true;
    },

    /**
             * 检查提交的数据
             * @return {boolean} true/false 是否合法
             */
    checkData() {
      const data = this.curLoadBalance;
      if (!data.cluster_id) {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('deploy.templateset.selectCluster'),
          delay: 2000,
        });
        return false;
      }

      if (!data.node_list || !data.node_list.length) {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('deploy.templateset.selectNodeIP'),
          delay: 2000,
        });
        return false;
      }

      if (!this.curLoadBalanceChartId || !this.chartVersionList.find(item => item.id === this.curLoadBalanceChartId)) {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('deploy.templateset.selectVer'),
          delay: 2000,
        });
        return false;
      }

      const values = data.values.trim();

      if (!values) {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('deploy.templateset.enterValuesContent'),
          delay: 2000,
        });
        return false;
      }

      try {
        yamljs.load(values);
      } catch (err) {
        this.$bkMessage({
          theme: 'error',
          message: this.$t('deploy.templateset.enterValidYAML'),
        });
        return false;
      }

      return true;
    },

    /**
             * 对接口返回的数据进行格式化以适应前端数据
             * @param  {array} list loadBalance列表
             * @return {array} list loadBalance列表
             */
    formatDataToClient(list) {
      list.forEach((item) => {
        item.namespace = item.namespace_id;
        item.protocol = {
          http: {
            port: 80,
            isUse: false,
          },
          https: {
            port: 443,
            isUse: false,
          },
        };

        // eg: http:8080;https:443;
        const protocols = item.protocol_type.split(';');
        protocols.forEach((protocol) => {
          const confs = protocol.split(':');
          if (['http', 'https'].includes(confs[0])) {
            item.protocol[confs[0]] = {
              port: confs[1],
              isUse: true,
            };
          }
        });

        // 例如"{"244":true}"
        const ipInfo = JSON.parse(item.ip_info);
        item.node_list = [];
        item.unsharedNum = 0;

        // eslint-disable-next-line no-restricted-syntax
        for (const key in ipInfo) {
          item.node_list.push({
            id: key,
            unshared: ipInfo[key],
          });
          if (ipInfo[key]) {
            // eslint-disable-next-line no-plusplus
            item.unsharedNum++;
          }
        }
        item.nodeNum = item.node_list.length;
      });
      return list;
    },

    /**
             * 对前端数据进行格式化以适应接口数据
             * @return {object} serverData serverData
             */
    formatDataToServer() {
      const data = this.curLoadBalance;
      const protocols = data.protocol;
      const nodeList = data.node_list;
      const nodeTmp = {};
      const serverData = {
        id: 0,
        name: data.name,
        project_id: data.project_id,
        cluster_id: data.cluster_id,
        namespace_id: data.namespace,
        protocol_type: '',
        ip_info: {},
        protocols: data.protocol,
      };

      if (data.id) {
        serverData.id = data.id;
      }

      if (protocols.http.isUse) {
        serverData.protocol_type = `http:${protocols.http.port}`;
      }

      if (protocols.https.isUse) {
        serverData.protocol_type += `;https:${protocols.https.port};`;
      }

      nodeList.forEach((node) => {
        nodeTmp[node.id] = node.unshared;
      });
      serverData.ip_info = JSON.stringify(nodeTmp);
      return serverData;
    },

    /**
             * 保存新建的LB
             */
    async createLoadBalance() {
      const { projectId } = this;

      const data = {
        project_id: projectId,
        cluster_id: this.curLoadBalance.cluster_id,
        values_content: this.curLoadBalance.values,
        ip_info: {},
        version: this.chartVersionList.find(item => item.id === this.curLoadBalanceChartId).version,
      };

      this.curLoadBalance.node_list.forEach((item) => {
        data.ip_info[String(`${item.id}`)] = false;
      });

      this.isDataSaveing = true;

      try {
        await this.$store.dispatch('network/addK8sLoadBalance', { projectId, data });
        this.searchScope = data.cluster_id;
        this.$bkMessage({
          theme: 'success',
          message: this.$t('generic.msg.success.save1'),
        });
        this.getLoadBalanceList();
        this.hideLoadBalanceSlider();
      } catch (e) {
        catchErrorHandler(e, this);
      } finally {
        this.isDataSaveing = false;
      }
    },

    /**
             * 保存更新的LB
             */
    async updateLoadBalance() {
      const { projectId } = this;

      const data = {
        project_id: projectId,
        cluster_id: this.curLoadBalance.cluster_id,
        values_content: this.curLoadBalance.values,
        ip_info: {},
        version: this.chartVersionList.find(item => item.id === this.curLoadBalanceChartId).version,
      };
      this.curLoadBalance.node_list.forEach((item) => {
        data.ip_info[String(`${item.id}`)] = false;
      });

      this.isDataSaveing = true;

      try {
        await this.$store.dispatch('network/updateLoadBalance', {
          projectId,
          loadBalanceId: this.curLoadBalance.id,
          data,
          projectKind: this.curProject.kind,
        });

        this.$bkMessage({
          theme: 'success',
          message: this.$t('generic.msg.success.save1'),
        });
        this.getLoadBalanceList();
        this.hideLoadBalanceSlider();
      } catch (e) {
        catchErrorHandler(e, this);
      } finally {
        this.isDataSaveing = false;
      }
    },

    /**
             * 保存LB
             */
    saveLoadBalance() {
      if (this.checkData()) {
        if (this.curLoadBalance.id) {
          this.updateLoadBalance();
        } else {
          this.createLoadBalance();
        }
      }
    },

    /**
             *  编辑器初始化之后的回调函数
             *  @param editor - 编辑器对象
             */
    editorInitAfter(editor) {
      this.aceEditor = editor;
    },
  },
};
</script>

<style scoped lang="postcss">
    @import '../../loadbalance.css';
    @import './index.css';
</style>
