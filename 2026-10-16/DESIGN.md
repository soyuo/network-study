# 동시성 작업 처리기 설계 문서

## 목표

독립적인 작업을 여러 Worker가 동시에 처리하면서, 정상 완료와 전체 취소
상황 모두에서 Worker와 결과 Channel이 안전하게 종료되도록 설계합니다.

## 처리 흐름

1. `Process`가 결과 Channel을 만들고 설정된 수만큼 Worker 고루틴을 시작합니다.
2. 각 Worker는 `ctx.Done()` 또는 작업 Channel 수신을 `select`로 기다립니다.
3. 사용자 작업을 호출하기 직전에 `ctx.Err()`를 다시 확인합니다. 이 검사는
   Buffer에 들어 있는 작업이 전체 취소 뒤에 새로 시작되는 상황을 줄입니다.
4. `execute`가 작업별 Timeout이 적용된 하위 Context를 만들고 작업을 실행합니다.
5. Worker가 결과를 결과 Channel로 보내고 다음 작업을 처리합니다.
6. 별도의 coordinator가 `sync.WaitGroup.Wait()`로 모든 Worker의 종료를 기다린
   다음 결과 Channel을 닫습니다. Worker는 결과 Channel을 직접 닫지 않습니다.

## Channel 소유권

입력 작업 Channel은 호출자가 소유합니다. 호출자는 모든 작업을 제출한 뒤
작업 Channel을 닫아야 합니다.

출력 결과 Channel은 Processor가 소유합니다. 모든 Worker가 종료된 것을
`WaitGroup`으로 확인한 coordinator만 결과 Channel을 정확히 한 번 닫습니다.
따라서 Worker가 보내는 중에 Channel이 닫히거나, 여러 고루틴이 동시에
Channel을 닫는 문제가 발생하지 않습니다.

## 전체 취소와 작업 Timeout

부모 Context가 취소되면 모든 Worker가 새 작업을 받지 않고 종료합니다.
각 작업에는 부모 취소와 개별 Timeout을 함께 반영하는 하위 Context가
전달됩니다.

Go는 임의의 작업 고루틴을 강제로 안전하게 종료할 수 없으므로, 작업 함수는
다음과 같은 협력적 취소 규칙을 따라야 합니다.

```go
select {
case <-ctx.Done():
    return nil, ctx.Err()
case value := <-someOperation:
    return value, nil
}
```

이 규칙을 지키면 작업 완료 또는 전체 취소 후 고루틴이 정상적으로 종료됩니다.

## Race 예방

Processor가 생성된 뒤 설정값은 변경되지 않습니다. Worker끼리 결과 Slice,
Map, 카운터를 공유하지 않고 결과는 Channel을 통해 전달합니다. Worker 종료는
`WaitGroup`으로 동기화하며, coordinator는 `Wait()`가 끝난 뒤에만 결과
Channel을 닫습니다. 테스트는 `go test -race ./...`로 데이터 경쟁을 검사합니다.

## 오류 처리

작업 함수가 반환한 오류는 `Result.Err`에 저장됩니다. Timeout 오류는
`ErrTaskTimeout`을 감싸며 작업 ID와 설정된 Timeout을 함께 표시합니다.
함수가 nil이면 오류로 처리하고, 작업 함수 내부의 Panic은 복구하여 오류로
변환합니다.
