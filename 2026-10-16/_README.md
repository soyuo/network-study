# 동시성 작업 처리기

여러 작업을 동시에 처리하기 위한 외부 라이브러리 없는 Go Worker Pool
엔진입니다. Channel, 설정 가능한 Worker 수, `sync.WaitGroup`, Context
취소, 작업별 Timeout, 안전한 종료, Race Detector 테스트를 포함합니다.

## 요구 사항

- Go 1.22 이상
- 표준 라이브러리만 사용

## 실행 방법

프로젝트 루트인 `1` 폴더에서 실행합니다.

```bash
go run .
go run ./cmd/demo
```

예제는 Worker 수를 1, 2, 4개로 바꾸어 실행 시간과 성공·실패 건수를
출력합니다. 각 작업은 50ms 동안 실행되므로 일반적으로 Worker 수가
늘어나면 전체 처리 시간이 줄어듭니다.

## 테스트 방법

```bash
go test ./...
go test -race ./...
go test -bench=. -benchtime=3x ./processor
```

`go test -race ./...`는 데이터 경쟁 여부를 검사하는 필수 검증 명령입니다.
Benchmark 결과는 실행하는 컴퓨터의 CPU와 시스템 상태에 따라 달라집니다.

## API와 설계

`processor.Task`는 작업 ID와 `context.Context`를 받는 작업 함수를 가집니다.
`Processor.Process(ctx, tasks)`는 읽기 전용 작업 Channel을 소비하고 결과
Channel을 반환합니다. 입력 작업 Channel은 호출자가 생성하고 닫습니다.
결과 Channel은 모든 Worker가 종료된 뒤 Processor가 닫습니다.

각 Worker는 사용자 작업을 시작하기 전에 전체 Context가 취소되었는지
확인합니다. 실제 작업에는 `TaskTimeout`이 적용된 하위 Context가 전달됩니다.
작업 함수는 `ctx.Done()`을 확인하여 취소에 빠르게 반응해야 합니다. Context를
무시하는 임의의 고루틴을 Go가 강제로 종료할 수는 없으므로, 작업 함수가
Context 취소를 협력적으로 처리한다는 규칙을 사용합니다.

## 동시성 문제 예방

- `sync.WaitGroup`이 설정된 Worker 수를 정확히 추적합니다.
- 모든 Worker가 종료된 뒤 coordinator 고루틴 하나만 `results`를 닫습니다.
- Worker는 Channel을 닫지 않으므로 send-after-close 문제가 발생하지 않습니다.
- 공유 Slice나 Map 대신 결과 Channel을 사용합니다.
- 전체 취소를 확인한 뒤에는 Buffer에 남은 작업을 새로 시작하지 않습니다.
- 작업 함수의 Panic을 Error로 변환하여 프로세스 전체가 종료되지 않게 합니다.
- Processor 내부에 Worker 간 공유하는 변경 가능한 상태를 두지 않습니다.

## 취소와 작업 큐

작업 Channel의 Buffer 크기를 지정하면 제한된 작업 큐로 사용할 수 있습니다.
큐가 가득 차면 생산자가 대기하게 하거나, 아래처럼 Context가 취소되었을
때 작업 제출을 포기하도록 만들 수 있습니다.

```go
select {
case tasks <- task:
case <-ctx.Done():
    return ctx.Err()
}
```

전체 취소 후 Worker는 새 작업을 받지 않습니다. 호출자는 반환된 결과
Channel이 닫힐 때까지 계속 읽어야 합니다.

## 파일 구성

- `processor/processor.go`: Worker Pool 구현과 공개 API
- `processor/processor_test.go`: 결과, 취소, Timeout, Panic, 고루틴 종료 테스트
- `processor/processor_benchmark_test.go`: Worker 수별 성능 Benchmark
- `cmd/demo/main.go`: 실행 예제와 처리 시간 비교
- `main.go`: 자동 채점기가 쉽게 실행할 수 있는 루트 진입점
- `DESIGN.md`: 상세한 설계와 동시성 예방 방법
