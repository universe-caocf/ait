# AIT Automation

`ait_automation` 是对当前 AIT 压测工具的一层自动化封装。它的目标不是替代 AIT，而是在不修改 AIT 源码的前提下，把下面这条人工链路变成可重复执行的自动流程：

1. 按多组参数反复执行 `go run ./cmd/ait`
2. 等待 AIT 压测结束并生成 `ait-report-*.json`
3. 自动读取新生成的 JSON 报告
4. 抽取关键指标并增量追加到 `benchmark_history.csv`
5. 在终端持续打印当前任务参数和工作进度

这套流程适合做批量对比压测、回归压测、不同并发组合实验，以及长期积累历史压测数据。

## 设计目标

- 不改动现有 AIT Go 代码
- 继续使用当前项目已有的报告能力
- 支持批量执行多组参数
- 支持把每次压测结果持续沉淀到一个总表
- 允许保留 AIT 原始 `ait-report-*.json` 文件，便于回溯
- 运行过程中能看到“现在正在压什么、压到哪里了”

## 目录结构

`ait_automation` 目录中的主要文件如下：

- [main.py](/Users/universe/Documents/projects/ait/ait_automation/main.py)：统一 CLI 入口
- [__main__.py](/Users/universe/Documents/projects/ait/ait_automation/__main__.py)：支持 `python3 -m ait_automation`
- [workflow.py](/Users/universe/Documents/projects/ait/ait_automation/workflow.py)：自动化总编排
- [benchmark_runner.py](/Users/universe/Documents/projects/ait/ait_automation/benchmark_runner.py)：批量运行 AIT
- [report_to_excel.py](/Users/universe/Documents/projects/ait/ait_automation/report_to_excel.py)：报告采集与汇总
- [config.example.yaml](/Users/universe/Documents/projects/ait/ait_automation/config.example.yaml)：示例配置

## 自动化流程原理

这套自动化流程是串行执行的。也就是说，它会按配置文件中的任务顺序，一轮一轮执行。

完整流程如下：

1. 入口脚本读取 YAML 配置  
   [main.py](/Users/universe/Documents/projects/ait/ait_automation/main.py) 接收 `--config` 参数，然后调用 [workflow.py](/Users/universe/Documents/projects/ait/ait_automation/workflow.py) 中的 `run_workflow()`。

2. 工作流解析任务列表  
   [workflow.py](/Users/universe/Documents/projects/ait/ait_automation/workflow.py) 支持两种方式生成任务：
   - `jobs`：显式写出每一组任务
   - `matrix + defaults`：自动展开模型、并发数、请求数的笛卡尔积

3. 执行单个压测任务  
   [benchmark_runner.py](/Users/universe/Documents/projects/ait/ait_automation/benchmark_runner.py) 会把 YAML 中的一组参数转换为一条真实的 AIT 命令，例如：

```bash
go run ./cmd/ait --model=deepseekv3_2 --concurrency=2 --count=10 --report=true --protocol=openai --baseUrl=http://127.0.0.1:38077/v1 --apiKey=dummy
```

4. 压测执行期间输出实时状态  
   运行器会在终端打印：
   - 当前第几轮、总共几轮
   - 当前模型、并发数、请求数
   - 实际执行命令
   - 定时心跳日志，例如 `running... elapsed=15s`

5. 压测结束后定位新报告  
   这是这个自动化模块里最关键的一步。

   AIT 当前源码会在压测结束后生成 `ait-report-<时间戳>.json` 和 `ait-report-<时间戳>.csv`。根据现有代码：
   - JSON 报告文件名由 AIT 内部固定生成
   - 报告写入位置是运行 AIT 时的当前工作目录
   - 生成发生在压测主流程结束之后

   自动化脚本的做法不是依赖终端输出文本去“猜”，而是做文件快照对比：
   - 执行前，先记录工作目录下已有的 `ait-report-*.json`
   - 执行后，再扫描一次
   - 用“执行后文件集合 - 执行前文件集合”得到本轮新增报告文件

   这样做的好处是：
   - 不依赖终端输出格式是否变化
   - 不需要修改 AIT 源码
   - 即使终端输出不完整，只要文件真的生成，就能准确找到

6. 采集新增 JSON，增量写入总表  
   [report_to_excel.py](/Users/universe/Documents/projects/ait/ait_automation/report_to_excel.py) 负责读取本轮新增的 JSON 报告，提取关键指标，然后追加写入 `benchmark_history.csv`。

7. 状态持久化，避免重复写入  
   采集器会维护一个状态文件，默认是 `.ait_report_ingest_state.json`。  
   它会记录哪些报告项已经写入过 CSV。这样即使你多次运行工作流，或者中途中断后重新执行，也不会把同一条结果反复写进总表。

## 为什么能判断“压测完成”

你之前提到的核心难点是：AIT 本身是生成 JSON 文件，而终端打印并不完整，那么脚本怎么知道什么时候一轮压测真的结束？

当前实现采用的是“双重判断”，其中主判断是进程结束：

- 第一层：AIT 子进程退出  
  `benchmark_runner.py` 使用 `subprocess.Popen()` 启动 `go run ./cmd/ait`，然后轮询 `proc.poll()`。只要子进程还没退出，说明该轮压测还在执行。

- 第二层：等待报告文件落盘  
  有些场景下，进程刚结束，文件系统上的报告文件可能还没在脚本下一次扫描时出现。因此运行器在进程结束后会再额外等待一个短窗口，默认 5 秒，继续检查是否出现新报告文件。

所以当前逻辑不是只靠“终端出现了某句文案”，而是：

1. AIT 进程退出
2. 再检查新 JSON 是否出现

这比单纯解析终端输出更稳。

## 为什么能定位报告文件位置

这是由 AIT 当前实现方式决定的：

- AIT 报告文件写到当前工作目录
- 自动化脚本运行 AIT 时显式指定了 `cwd`
- 所以报告文件一定出现在工作流配置中的 `runner.workdir` 下

这也是为什么配置里有这个字段：

```yaml
runner:
  workdir: "."
```

如果你以后想把压测和报告统一放到某个目录，只需要把 `workdir` 改成目标目录即可。

## 功能说明

当前自动化模块支持以下能力：

- 批量运行 AIT 压测
- 为每轮任务自动添加 `--report=true`
- 支持不同模型、不同并发数、不同请求数
- 支持把公共参数写在 `defaults`
- 支持手写任务列表 `jobs`
- 支持矩阵模式自动展开任务
- 自动检测新生成的 `ait-report-*.json`
- 自动提取关键性能指标
- 自动追加写入 `benchmark_history.csv`
- 自动去重，避免重复写入历史结果
- 保留原始 `ait-report-*.json` 和 AIT 自己生成的 CSV 报告
- 终端打印实时进度
- 支持 `--dry-run` 先预览计划，不实际执行

## 模块职责详解

### 1. 入口层

[main.py](/Users/universe/Documents/projects/ait/ait_automation/main.py) 的职责很简单：

- 接收命令行参数
- 读取 `--config`
- 读取 `--dry-run`
- 调用工作流入口

你可以通过两种方式启动：

```bash
python3 -m ait_automation --config ait_automation/config.example.yaml
```

或者：

```bash
python3 ait_automation/main.py --config ait_automation/config.example.yaml
```

### 2. 工作流编排层

[workflow.py](/Users/universe/Documents/projects/ait/ait_automation/workflow.py) 是整个自动化流程的核心调度器。

它负责：

- 读取 YAML 配置
- 校验配置结构是否正确
- 构建压测任务列表
- 初始化运行器和报告采集器
- 按顺序执行每个压测任务
- 在每轮任务结束后，把新报告写入总表
- 输出整个工作流的汇总结果

### 3. 压测执行层

[benchmark_runner.py](/Users/universe/Documents/projects/ait/ait_automation/benchmark_runner.py) 专门负责“跑 AIT”。

它做的事情包括：

- 把任务参数转成 AIT 命令行参数
- 启动 `go run ./cmd/ait`
- 在执行前后扫描 `ait-report-*.json`
- 通过前后快照差集找出新增报告
- 打印当前进度和当前参数

### 4. 报告采集层

[report_to_excel.py](/Users/universe/Documents/projects/ait/ait_automation/report_to_excel.py) 现在已经不只是一个独立脚本，它也被设计成了可复用模块。

其中最重要的类是 `ReportCollector`，它负责：

- 读取 JSON 报告
- 解析 `models` 字段中的每条结果
- 抽取你关心的字段
- 追加写入 CSV 或 XLSX
- 管理去重状态

## 默认采集了哪些指标

当前默认字段定义在 [report_to_excel.py](/Users/universe/Documents/projects/ait/ait_automation/report_to_excel.py) 的 `DEFAULT_FIELDS` 中，主要包括：

- `timestamp`
- `model`
- `protocol`
- `base_url`
- `target_ip`
- `total_requests`
- `concurrency`
- `is_stream`
- `is_thinking`
- `success_rate`
- `error_rate`
- `avg_total_time`
- `avg_ttft`
- `avg_tpot`
- `avg_tps`
- `avg_total_throughput_tps`
- `system_output_tps`
- `system_total_tps`
- `total_output_tokens`

这些字段基本覆盖了：

- 基础压测参数
- 成功率/错误率
- 延迟指标
- 输出速率指标
- 系统吞吐指标

如果后续你想扩展更多列，只需要修改 YAML 中的 `history.fields` 即可，不需要改 AIT Go 源码。

## 配置文件说明

自动化流程通过 YAML 文件驱动，示例见 [config.example.yaml](/Users/universe/Documents/projects/ait/ait_automation/config.example.yaml)。

### `runner`

用于定义如何运行 AIT：

```yaml
runner:
  command: ["go", "run", "./cmd/ait"]
  workdir: "."
  env:
    OPENAI_API_KEY: "xxx"
    OPENAI_BASE_URL: "http://127.0.0.1:38077/v1"
  report_pattern: "ait-report-*.json"
```

字段说明：

- `command`：启动 AIT 的命令，当前默认就是你使用的 `go run ./cmd/ait`
- `workdir`：AIT 的执行目录，也是报告文件的扫描目录
- `env`：运行时附加环境变量
- `report_pattern`：扫描 JSON 报告时使用的文件匹配模式

### `history`

用于定义总表输出：

```yaml
history:
  output: "benchmark_history.csv"
  state_file: ".ait_report_ingest_state.json"
  fields:
    - timestamp
    - model
    - concurrency
    - total_requests
    - avg_ttft
    - avg_tps
```

字段说明：

- `output`：总表路径，推荐用 CSV
- `state_file`：去重状态文件
- `fields`：写入总表的列顺序

### `jobs`

适合你想精确指定每一轮任务的情况：

```yaml
jobs:
  - model: "deepseekv3_2"
    concurrency: 1
    count: 5
    protocol: "openai"
    base_url: "http://127.0.0.1:38077/v1"
    api_key: "dummy"
    stream: true
    thinking: false
    timeout: 300
    prompt_length: 100

  - model: "deepseekv3_2"
    concurrency: 2
    count: 10
    protocol: "openai"
    base_url: "http://127.0.0.1:38077/v1"
    api_key: "dummy"
    stream: true
    thinking: false
    timeout: 300
    prompt_length: 100
```

### `defaults + matrix`

适合你想批量组合并发数和请求数时使用：

```yaml
defaults:
  protocol: "openai"
  base_url: "http://127.0.0.1:38077/v1"
  api_key: "dummy"
  stream: true
  thinking: false
  timeout: 300
  prompt_length: 100

matrix:
  models: ["deepseekv3_2"]
  concurrencies: [1, 2, 4]
  counts: [5, 10]
```

这会自动展开为 6 个任务：

- 1 并发，5 请求
- 1 并发，10 请求
- 2 并发，5 请求
- 2 并发，10 请求
- 4 并发，5 请求
- 4 并发，10 请求

### `defaults + matrix.pairs`（推荐用于定制化组合）

当你不希望跑“笛卡尔积全排列”，而是希望显式指定“哪些并发数对应哪些请求数”时，用 `matrix.pairs` 更合适：

```yaml
defaults:
  protocol: "openai"
  base_url: "http://127.0.0.1:38077/v1"
  api_key: "dummy"
  stream: true
  thinking: false
  timeout: 300
  prompt_length: 100

matrix:
  models: ["deepseekv3_2"]
  pairs:
    - concurrency: 1
      count: 5
    - concurrency: 2
      count: 10
    - concurrency: 4
      count: 10
```

这会展开为 3 个任务（按 `pairs` 顺序执行）：

- 1 并发，5 请求
- 2 并发，10 请求
- 4 并发，10 请求

注意：`matrix.pairs` 与 `matrix.concurrencies/counts` 是互斥的，不能混用。

## 使用方式

### 安装依赖

YAML 解析依赖：

```bash
pip install pyyaml
```

如果你要输出 `.xlsx`，还需要：

```bash
pip install openpyxl
```

### 预览任务，不实际执行

```bash
python3 -m ait_automation --config ait_automation/config.example.yaml --dry-run
```

这个模式适合先确认：

- 一共会跑多少轮
- 每一轮的模型、并发数、请求数是什么
- 配置是否写对

### 执行完整自动化流程

```bash
python3 -m ait_automation --config ait_automation/config.example.yaml
```

执行时你会看到类似日志：

```text
[WORKFLOW] config=/path/to/config.yaml
[WORKFLOW] workdir=/path/to/ait
[WORKFLOW] total_jobs=6
[WORKFLOW] history_csv=/path/to/benchmark_history.csv

[RUN 1/6] start: model=deepseekv3_2, concurrency=1, count=5, stream=True, thinking=False
[RUN 1/6] command: go run ./cmd/ait --model=deepseekv3_2 --concurrency=1 --count=5 --report=true ...
[RUN 1/6] running... elapsed=15s (model=deepseekv3_2, concurrency=1, count=5)
[RUN 1/6] done: rc=0, elapsed=42.3s, new_json_reports=1
[RUN 1/6] report: /path/to/ait-report-26-03-20-10-00-00.json
[WORKFLOW] run 1/6 ingested 1 row(s), history_total_added_this_workflow=1
```

## 输出文件说明

执行结束后，通常会看到三类结果：

1. AIT 原始报告文件  
   例如：
   - `ait-report-26-03-20-10-00-00.json`
   - `ait-report-26-03-20-10-00-00.csv`

2. 自动化历史总表  
   例如：
   - `benchmark_history.csv`

3. 自动化去重状态文件  
   例如：
   - `.ait_report_ingest_state.json`

其中：

- 原始 JSON/CSV 是 AIT 自己生成的，不会被删除
- `benchmark_history.csv` 是自动化层维护的统一汇总表
- 状态文件只是为了防重复写入，不是最终分析产物

## Linux 下为什么推荐 CSV

你之前担心 Linux 下“没有 Excel 文件格式”怎么做。这里要明确一点：

- Linux 不需要安装 Microsoft Excel 才能处理表格数据
- CSV 是最稳妥的跨平台格式
- Excel、WPS、LibreOffice Calc 都能直接打开 CSV

所以当前默认输出 `benchmark_history.csv` 是最合适的。  
如果你确实需要 `.xlsx`，采集模块也支持，但要安装 `openpyxl`。

## 当前实现的边界

这套脚本已经可以稳定完成自动化主流程，但有几个边界需要你心里有数：

- 当前是串行执行任务，不是并行批量跑多个 AIT 进程
- “一轮任务对应一份新 JSON 报告”的假设依赖 AIT 当前实现
- 如果 AIT 进程退出了但没有生成 JSON，工作流会提示本轮没有可采集报告
- `PyYAML` 不是 Python 标准库，需要额外安装

## 建议使用习惯

- 先用 `--dry-run` 检查任务展开是否符合预期
- 优先使用 `benchmark_history.csv` 作为历史趋势总表
- 保留 `ait-report-*.json`，后续排查异常时很有价值
- 如果你会频繁重复做同一类实验，建议为不同实验场景单独建 YAML 文件

## 总结

`ait_automation` 的核心价值在于把 AIT 从“手工一次次执行的压测工具”，变成“可批量、可追踪、可沉淀历史结果的自动化压测流程”。

它的关键实现点有三个：

- 用 YAML 管理多组压测参数
- 用进程结束 + 文件快照差集来可靠识别每轮压测结果
- 用增量写入 + 去重状态来持续维护 `benchmark_history.csv`

如果你后面希望，我还可以继续把这份 README 再补两类内容：

1. 增加“真实业务场景”的 YAML 示例
2. 增加“常见问题排查”章节，比如为什么没有生成 JSON、为什么没有写入总表
