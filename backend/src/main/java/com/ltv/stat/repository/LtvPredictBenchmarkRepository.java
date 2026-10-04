package com.ltv.stat.repository;

import com.ltv.stat.entity.LtvPredictBenchmark;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Repository
public interface LtvPredictBenchmarkRepository extends JpaRepository<LtvPredictBenchmark, Long> {
    List<LtvPredictBenchmark> findByDimensionTypeAndDimensionValueAndSubPeriodDaysOrderByDayIndexAsc(
            String dimensionType, String dimensionValue, Integer subPeriodDays);

    @Modifying(clearAutomatically = true)
    @Transactional
    @Query("DELETE FROM LtvPredictBenchmark b WHERE b.dimensionType = :dimensionType AND b.dimensionValue = :dimensionValue AND b.subPeriodDays = :subPeriodDays")
    void deleteByDimensionTypeAndDimensionValueAndSubPeriodDays(
            @Param("dimensionType") String dimensionType,
            @Param("dimensionValue") String dimensionValue,
            @Param("subPeriodDays") Integer subPeriodDays);
}
