import { CONFIG_FILE_TYPE } from '../constants/config';

// 查询配置文件类型名称
export const getConfigTypeName = (type: string) => {
  const fileType = CONFIG_FILE_TYPE.find((item) => item.id === type);
  return fileType ? fileType.name : '未知格式';
};

export function getDefaultConfigItem() {
  return {
    id: 0,
    spec: {
      file_mode: '',
      file_type: '',
      memo: '',
      name: '',
      path: '',
      permission: {
        privilege: '644',
        user: 'root',
        user_group: 'root',
      },
    },
    attachment: {
      biz_id: 0,
      app_id: 0,
    },
    revision: {
      creator: '',
      create_at: '',
      reviser: '',
      update_at: '',
    },
  };
}

// 配置文件编辑参数
export function getConfigEditParams() {
  return {
    name: '',
    memo: '',
    path: '',
    file_type: 'text',
    file_mode: 'unix',
    user: 'root',
    user_group: 'root',
    privilege: '644',
  };
}
