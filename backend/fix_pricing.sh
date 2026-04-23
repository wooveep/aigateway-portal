sed -i 's/InputPer1K/InputCostPerMillionTokens \/ 1000/g' internal/service/portal/*.go
sed -i 's/OutputPer1K/OutputCostPerMillionTokens \/ 1000/g' internal/service/portal/*.go
sed -i 's/\.InputCostPerToken/\.InputCostPerMillionTokens \/ 1000000/g' internal/service/portal/*.go
sed -i 's/\.OutputCostPerToken/\.OutputCostPerMillionTokens \/ 1000000/g' internal/service/portal/*.go
